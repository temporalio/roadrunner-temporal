package rrtemporal

import (
	"context"
	stderr "errors"
	"os"
	"time"

	"connectrpc.com/connect"
	commonV1 "github.com/roadrunner-server/api-go/v6/common/v1"
	protoApi "github.com/roadrunner-server/api-go/v6/temporal/v1"
	"github.com/roadrunner-server/api-go/v6/temporal/v1/temporalV1connect"
	"github.com/roadrunner-server/errors"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/activity"
	tlog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// rpc exposes the temporal plugin control API (temporal.v1.TemporalService)
// on the RoadRunner Connect-RPC plane.
type rpc struct {
	plugin *Plugin
}

// Compile-time check that rpc implements the generated handler interface.
var _ temporalV1connect.TemporalServiceHandler = (*rpc)(nil)

// newStatus builds the soft-error status carried inside replay responses
// (kept in the response body for parity with the v5 RPC behavior).
func newStatus(code codes.Code, msg string) *commonV1.Status {
	// gRPC status codes are tiny (0..16), the conversion cannot overflow
	return &commonV1.Status{Code: int32(code), Message: msg} //nolint:gosec
}

// RecordActivityHeartbeat records a heartbeat for an activity.
// task_token - is the value of the binary "TaskToken" field of the "ActivityInfo" struct retrieved inside the activity.
// details - is the progress you want to record along with the heartbeat for this activity.
func (r *rpc) RecordActivityHeartbeat(_ context.Context, req *connect.Request[protoApi.RecordHeartbeatRequest]) (*connect.Response[protoApi.RecordHeartbeatResponse], error) {
	details := &commonpb.Payloads{}

	if len(req.Msg.GetDetails()) != 0 {
		if err := proto.Unmarshal(req.Msg.GetDetails(), details); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}

	if r.plugin.getActDef() == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.Str("no activity definition registered"))
	}

	// find running activity
	r.plugin.mu.RLock()
	ctx, err := r.plugin.temporal.rrActivityDef.GetActivityContext(req.Msg.GetTaskToken())
	if err != nil {
		r.plugin.mu.RUnlock()
		return nil, err
	}
	r.plugin.mu.RUnlock()

	activity.RecordHeartbeat(ctx, details)

	err = context.Cause(ctx)
	if err != nil {
		if stderr.Is(err, activity.ErrActivityPaused) {
			return connect.NewResponse(&protoApi.RecordHeartbeatResponse{Paused: true}), nil
		}
	}

	out := &protoApi.RecordHeartbeatResponse{}
	select {
	case <-ctx.Done():
		out.Canceled = true
	default:
		out.Canceled = false
	}

	return connect.NewResponse(out), nil
}

func (r *rpc) GetActivityNames(_ context.Context, _ *connect.Request[protoApi.GetNamesRequest]) (*connect.Response[protoApi.NamesList], error) {
	r.plugin.mu.RLock()
	defer r.plugin.mu.RUnlock()

	out := &protoApi.NamesList{Names: make([]string, 0, len(r.plugin.temporal.activities))}
	for k := range r.plugin.temporal.activities {
		out.Names = append(out.Names, k)
	}

	return connect.NewResponse(out), nil
}

func (r *rpc) GetWorkflowNames(_ context.Context, _ *connect.Request[protoApi.GetNamesRequest]) (*connect.Response[protoApi.NamesList], error) {
	r.plugin.mu.RLock()
	defer r.plugin.mu.RUnlock()

	out := &protoApi.NamesList{Names: make([]string, 0, len(r.plugin.temporal.workflows))}
	for k := range r.plugin.temporal.workflows {
		out.Names = append(out.Names, k)
	}

	return connect.NewResponse(out), nil
}

func (r *rpc) ReplayWorkflow(_ context.Context, req *connect.Request[protoApi.ReplayRequest]) (*connect.Response[protoApi.ReplayResponse], error) {
	in := req.Msg
	out := &protoApi.ReplayResponse{}

	r.plugin.log.Debug("replay workflow request",
		"run_id", in.GetWorkflowExecution().GetRunId(),
		"workflow_id", in.GetWorkflowExecution().GetWorkflowId(),
		"workflow_name", in.GetWorkflowType().GetName())

	if in.GetWorkflowExecution() == nil || in.GetWorkflowType() == nil ||
		in.GetWorkflowExecution().GetRunId() == "" || in.GetWorkflowExecution().GetWorkflowId() == "" || in.GetWorkflowType().GetName() == "" {
		out.Status = newStatus(codes.InvalidArgument, "run_id, workflow_id or workflow_name should not be empty")

		r.plugin.log.Error("replay workflow request", "error", "run_id, workflow_id or workflow_name should not be empty")
		return connect.NewResponse(out), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	iter := r.plugin.temporal.client.GetWorkflowHistory(ctx, in.GetWorkflowExecution().GetWorkflowId(), in.GetWorkflowExecution().GetRunId(), false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)

	var hist history.History
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			out.Status = newStatus(codes.Internal, err.Error())

			r.plugin.log.Error("history iteration error", "error", err)
			return connect.NewResponse(out), nil
		}
		hist.Events = append(hist.Events, event)
	}

	if r.plugin.getWfDef() == nil {
		out.Status = newStatus(codes.FailedPrecondition, "workflow definition is not initialized, retry in a second")

		return connect.NewResponse(out), nil
	}

	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(r.plugin.getWfDef(), workflow.RegisterOptions{
		Name:                          in.GetWorkflowType().GetName(),
		DisableAlreadyRegisteredCheck: false,
	})

	err := replayer.ReplayWorkflowHistory(tlog.NewStructuredLogger(r.plugin.log), &hist)
	if err != nil {
		out.Status = newStatus(codes.FailedPrecondition, err.Error())

		r.plugin.log.Error("replay error", "error", err)
		return connect.NewResponse(out), nil
	}

	out.Status = newStatus(codes.OK, "")

	r.plugin.log.Debug("replay workflow request finished successfully")

	return connect.NewResponse(out), nil
}

func (r *rpc) DownloadWorkflowHistory(_ context.Context, req *connect.Request[protoApi.ReplayRequest]) (*connect.Response[protoApi.ReplayResponse], error) {
	in := req.Msg
	out := &protoApi.ReplayResponse{}

	r.plugin.log.Debug("replay workflow request",
		"run_id", in.GetWorkflowExecution().GetRunId(),
		"workflow_id", in.GetWorkflowExecution().GetWorkflowId(),
		"save_path", in.GetSavePath())

	if in.GetWorkflowExecution() == nil || in.GetWorkflowType() == nil || in.GetSavePath() == "" ||
		in.GetWorkflowExecution().GetRunId() == "" || in.GetWorkflowExecution().GetWorkflowId() == "" || in.GetWorkflowType().GetName() == "" {
		out.Status = newStatus(codes.InvalidArgument, "run_id, workflow_id or save_path should not be empty")

		r.plugin.log.Error("replay workflow request", "error", "run_id, workflow_id or save_path should not be empty")
		return connect.NewResponse(out), nil
	}

	file, err := os.Create(in.GetSavePath())
	if err != nil {
		out.Status = newStatus(codes.Internal, err.Error())

		r.plugin.log.Error("failed to create the file", "error", err)
		return connect.NewResponse(out), nil
	}

	defer func() {
		err = file.Close()
		if err != nil {
			r.plugin.log.Error("failed to close the file", "error", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	iter := r.plugin.temporal.client.GetWorkflowHistory(ctx, in.GetWorkflowExecution().GetWorkflowId(), in.GetWorkflowExecution().GetRunId(), false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)

	var hist history.History

	for iter.HasNext() {
		event, errn := iter.Next()
		if errn != nil {
			out.Status = newStatus(codes.Internal, errn.Error())

			r.plugin.log.Error("history iteration error", "error", errn)
			return connect.NewResponse(out), nil
		}

		hist.Events = append(hist.Events, event)
	}

	data, err := protojson.Marshal(&hist)
	if err != nil {
		out.Status = newStatus(codes.Internal, err.Error())

		r.plugin.log.Error("history marshal error", "error", err)
		return connect.NewResponse(out), nil
	}

	_, err = file.Write(data)
	if err != nil {
		out.Status = newStatus(codes.Internal, err.Error())

		r.plugin.log.Error("history marshal error", "error", err)
		return connect.NewResponse(out), nil
	}

	out.Status = newStatus(codes.OK, "")

	r.plugin.log.Debug("history saved", "location", in.GetSavePath())

	return connect.NewResponse(out), nil
}

func (r *rpc) ReplayFromJSON(_ context.Context, req *connect.Request[protoApi.ReplayRequest]) (*connect.Response[protoApi.ReplayResponse], error) {
	in := req.Msg
	out := &protoApi.ReplayResponse{}

	r.plugin.log.Debug("replay from JSON request",
		"workflow_name", in.GetWorkflowType().GetName(),
		"save_path", in.GetSavePath(),
		"last_event_id", in.GetLastEventId(),
	)

	if in.GetWorkflowType() == nil || in.GetSavePath() == "" {
		out.Status = newStatus(codes.InvalidArgument, "workflow_name and save_path should not be empty")

		r.plugin.log.Error("replay from JSON request", "error", "workflow_name and save_path should not be empty")
		return connect.NewResponse(out), nil
	}

	if in.GetWorkflowType().GetName() == "" {
		out.Status = newStatus(codes.InvalidArgument, "workflow_name should not be empty")

		r.plugin.log.Error("replay from JSON request", "error", "workflow_name should not be empty")
		return connect.NewResponse(out), nil
	}

	if r.plugin.getWfDef() == nil {
		out.Status = newStatus(codes.FailedPrecondition, "workflow definition is not initialized, retry in a second")

		return connect.NewResponse(out), nil
	}

	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(r.plugin.getWfDef(), workflow.RegisterOptions{
		Name:                          in.GetWorkflowType().GetName(),
		DisableAlreadyRegisteredCheck: false,
	})

	switch in.GetLastEventId() {
	// we don't have last event ID
	case 0:
		err := replayer.ReplayWorkflowHistoryFromJSONFile(tlog.NewStructuredLogger(r.plugin.log), in.GetSavePath())
		if err != nil {
			out.Status = newStatus(codes.FailedPrecondition, err.Error())

			r.plugin.log.Error("replay from JSON request", "error", err)
			return connect.NewResponse(out), nil
		}
	default:
		// we have last event ID
		err := replayer.ReplayPartialWorkflowHistoryFromJSONFile(tlog.NewStructuredLogger(r.plugin.log), in.GetSavePath(), in.GetLastEventId())
		if err != nil {
			out.Status = newStatus(codes.FailedPrecondition, err.Error())

			r.plugin.log.Error("replay from JSON request (partial workflow history)", "id", in.GetLastEventId(), "error", err)
			return connect.NewResponse(out), nil
		}
	}

	out.Status = newStatus(codes.OK, "")

	r.plugin.log.Debug("replay from JSON request finished successfully")

	return connect.NewResponse(out), nil
}

func (r *rpc) ReplayWorkflowHistory(_ context.Context, req *connect.Request[protoApi.History]) (*connect.Response[protoApi.ReplayResponse], error) {
	in := req.Msg
	out := &protoApi.ReplayResponse{}

	r.plugin.log.Debug("replay from workflow history request",
		"workflow_name", in.GetWorkflowType().GetName(),
	)

	if in.GetHistory() == nil || in.GetWorkflowType().GetName() == "" {
		out.Status = newStatus(codes.FailedPrecondition, "workflow_name and/or history should not be empty")

		r.plugin.log.Error("workflow_name and/or history should not be empty")
		return connect.NewResponse(out), nil
	}

	if r.plugin.getWfDef() == nil {
		out.Status = newStatus(codes.FailedPrecondition, "workflow definition is not initialized, retry in a second")

		return connect.NewResponse(out), nil
	}

	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(r.plugin.getWfDef(), workflow.RegisterOptions{
		Name:                          in.GetWorkflowType().GetName(),
		DisableAlreadyRegisteredCheck: false,
	})

	err := replayer.ReplayWorkflowHistory(tlog.NewStructuredLogger(r.plugin.log), in.GetHistory())
	if err != nil {
		out.Status = newStatus(codes.FailedPrecondition, err.Error())

		r.plugin.log.Error("replay workflow history", "error", err)
		return connect.NewResponse(out), nil
	}

	out.Status = newStatus(codes.OK, "")

	r.plugin.log.Debug("replay workflow request finished successfully")

	return connect.NewResponse(out), nil
}

func (r *rpc) UpdateAPIKey(_ context.Context, req *connect.Request[protoApi.UpdateAPIKeyRequest]) (*connect.Response[protoApi.UpdateAPIKeyResponse], error) {
	if key := req.Msg.GetApiKey(); key != "" {
		r.plugin.apiKey.Store(&key)
		return connect.NewResponse(&protoApi.UpdateAPIKeyResponse{Ok: true}), nil
	}

	return connect.NewResponse(&protoApi.UpdateAPIKeyResponse{Ok: false}), nil
}
