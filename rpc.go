package rrtemporal

import (
	"context"
	stderr "errors"
	"os"
	"time"

	commonV1 "github.com/roadrunner-server/api-go/v6/common/v1"
	protoApi "github.com/roadrunner-server/api-go/v6/temporal/v1"
	"github.com/roadrunner-server/errors"
	"github.com/temporalio/roadrunner-temporal/v6/internal/logger"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

/*
- the method's type is exported.
- the method is exported.
- the method has two arguments, both exported (or builtin) types.
- the method's second argument is a pointer.
- the method has return type error.
*/
type rpc struct {
	plugin *Plugin
	client client.Client
}

// RecordHeartbeatRequest sent by activity to record current state.
type RecordHeartbeatRequest struct {
	TaskToken []byte `json:"taskToken"`
	Details   []byte `json:"details"`
}

// RecordHeartbeatResponse sent back to the worker to indicate that activity was canceled.
type RecordHeartbeatResponse struct {
	Canceled bool `json:"canceled"`
	Paused   bool `json:"paused"`
}

// RecordActivityHeartbeat records heartbeat for an activity.
// taskToken - is the value of the binary "TaskToken" field of the "ActivityInfo" struct retrieved inside the activity.
// details - is the progress you want to record along with heart beat for this activity.
// The errors it can return:
// - EntityNotExistsError
// - InternalServiceError
// - CanceledError
func (r *rpc) RecordActivityHeartbeat(in RecordHeartbeatRequest, out *RecordHeartbeatResponse) error {
	details := &commonpb.Payloads{}

	if len(in.Details) != 0 {
		if err := proto.Unmarshal(in.Details, details); err != nil {
			return err
		}
	}

	if r.plugin.getActDef() == nil {
		return errors.Str("no activity definition registered")
	}

	// find running activity
	ctx, err := r.plugin.getActDef().GetActivityContext(in.TaskToken)
	if err != nil {
		return err
	}

	activity.RecordHeartbeat(ctx, details)

	err = context.Cause(ctx)
	if err != nil {
		if stderr.Is(err, activity.ErrActivityPaused) {
			*out = RecordHeartbeatResponse{Paused: true}
			return nil
		}
	}

	select {
	case <-ctx.Done():
		*out = RecordHeartbeatResponse{Canceled: true}
	default:
		*out = RecordHeartbeatResponse{Canceled: false}
	}

	return nil
}

func (r *rpc) GetActivityNames(_ bool, out *[]string) error {
	r.plugin.mu.RLock()
	defer r.plugin.mu.RUnlock()
	for k := range r.plugin.temporal.activities {
		*out = append(*out, k)
	}
	return nil
}

func (r *rpc) GetWorkflowNames(_ bool, out *[]string) error {
	r.plugin.mu.RLock()
	defer r.plugin.mu.RUnlock()

	for k := range r.plugin.temporal.workflows {
		*out = append(*out, k)
	}

	return nil
}

func (r *rpc) ReplayWorkflow(in *protoApi.ReplayRequest, out *protoApi.ReplayResponse) error {
	r.plugin.log.Debug("replay workflow request",
		"run_id", in.GetWorkflowExecution().GetRunId(),
		"workflow_id", in.GetWorkflowExecution().GetWorkflowId(),
		"workflow_name", in.GetWorkflowType().GetName())

	if in.GetWorkflowExecution() == nil || in.GetWorkflowType() == nil {
		out.Status = &commonV1.Status{
			Code:    int32(codes.InvalidArgument),
			Message: "run_id, workflow_id or workflow_name should not be empty",
		}

		r.plugin.log.Error("replay workflow request", "error", "run_id, workflow_id or workflow_name should not be empty")
		return nil
	}

	if in.GetWorkflowExecution().GetRunId() == "" || in.GetWorkflowExecution().GetWorkflowId() == "" || in.GetWorkflowType().GetName() == "" {
		out.Status = &commonV1.Status{
			Code:    int32(codes.InvalidArgument),
			Message: "run_id, workflow_id or workflow_name should not be empty",
		}

		r.plugin.log.Error("replay workflow request", "error", "run_id, workflow_id or workflow_name should not be empty")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	iter := r.plugin.temporal.client.GetWorkflowHistory(ctx, in.GetWorkflowExecution().GetWorkflowId(), in.GetWorkflowExecution().GetRunId(), false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)

	var hist history.History
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			out.Status = &commonV1.Status{
				Code:    int32(codes.Internal),
				Message: err.Error(),
			}

			r.plugin.log.Error("history iteration error", "error", err)
			return nil
		}
		hist.Events = append(hist.Events, event)
	}

	if r.plugin.getWfDef() == nil {
		out.Status = &commonV1.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: "workflow definition is not initialized, retry in a second",
		}

		return nil
	}

	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(r.plugin.getWfDef(), workflow.RegisterOptions{
		Name:                          in.GetWorkflowType().GetName(),
		DisableAlreadyRegisteredCheck: false,
	})

	err := replayer.ReplayWorkflowHistory(logger.NewSlogAdapter(r.plugin.log), &hist)
	if err != nil {
		out.Status = &commonV1.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: err.Error(),
		}

		r.plugin.log.Error("replay error", "error", err)
		return nil
	}

	out.Status = &commonV1.Status{
		Code: int32(codes.OK),
	}

	r.plugin.log.Debug("replay workflow request finished successfully")

	return nil
}

func (r *rpc) DownloadWorkflowHistory(in *protoApi.ReplayRequest, out *protoApi.ReplayResponse) error {
	r.plugin.log.Debug("replay workflow request",
		"run_id", in.GetWorkflowExecution().GetRunId(),
		"workflow_id", in.GetWorkflowExecution().GetWorkflowId(),
		"save_path", in.GetSavePath())

	if in.GetWorkflowExecution() == nil || in.GetWorkflowType() == nil || in.GetSavePath() == "" {
		out.Status = &commonV1.Status{
			Code:    int32(codes.InvalidArgument),
			Message: "run_id, workflow_id or save_path should not be empty",
		}

		return nil
	}

	if in.GetWorkflowExecution().GetRunId() == "" || in.GetWorkflowExecution().GetWorkflowId() == "" || in.GetWorkflowType().GetName() == "" {
		out.Status = &commonV1.Status{
			Code:    int32(codes.InvalidArgument),
			Message: "run_id, workflow_id or save_path should not be empty",
		}

		r.plugin.log.Error("replay workflow request", "error", "run_id, workflow_id or save_path should not be empty")
		return nil
	}

	file, err := os.Create(in.GetSavePath())
	if err != nil {
		out.Status = &commonV1.Status{
			Code:    int32(codes.Internal),
			Message: err.Error(),
		}

		r.plugin.log.Error("failed to create the file", "error", err)
		return nil
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
			out.Status = &commonV1.Status{
				Code:    int32(codes.Internal),
				Message: errn.Error(),
			}

			r.plugin.log.Error("history iteration error", "error", errn)
			return nil
		}

		hist.Events = append(hist.Events, event)
	}

	data, err := protojson.Marshal(&hist)
	if err != nil {
		out.Status = &commonV1.Status{
			Code:    int32(codes.Internal),
			Message: err.Error(),
		}

		r.plugin.log.Error("history marshal error", "error", err)
		return nil
	}

	_, err = file.Write(data)
	if err != nil {
		out.Status = &commonV1.Status{
			Code:    int32(codes.Internal),
			Message: err.Error(),
		}

		r.plugin.log.Error("history marshal error", "error", err)
		return nil
	}

	out.Status = &commonV1.Status{
		Code: int32(codes.OK),
	}

	r.plugin.log.Debug("history saved", "location", in.GetSavePath())

	return nil
}

func (r *rpc) ReplayFromJSON(in *protoApi.ReplayRequest, out *protoApi.ReplayResponse) error {
	r.plugin.log.Debug("replay from JSON request",
		"workflow_name", in.GetWorkflowType().GetName(),
		"save_path", in.GetSavePath(),
		"last_event_id", in.GetLastEventId(),
	)

	if in.GetWorkflowType() == nil || in.GetSavePath() == "" {
		out.Status = &commonV1.Status{
			Code:    int32(codes.InvalidArgument),
			Message: "workflow_name and save_path should not be empty",
		}

		r.plugin.log.Error("replay from JSON request", "error", "workflow_name and save_path should not be empty")
		return nil
	}

	if in.GetWorkflowType().GetName() == "" {
		out.Status = &commonV1.Status{
			Code:    int32(codes.InvalidArgument),
			Message: "workflow_name should not be empty",
		}

		r.plugin.log.Error("replay from JSON request", "error", "workflow_name should not be empty")
		return nil
	}

	if r.plugin.getWfDef() == nil {
		out.Status = &commonV1.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: "workflow definition is not initialized, retry in a second",
		}

		return nil
	}

	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(r.plugin.getWfDef(), workflow.RegisterOptions{
		Name:                          in.GetWorkflowType().GetName(),
		DisableAlreadyRegisteredCheck: false,
	})

	switch in.GetLastEventId() {
	// we don't have last event ID
	case 0:
		err := replayer.ReplayWorkflowHistoryFromJSONFile(logger.NewSlogAdapter(r.plugin.log), in.GetSavePath())
		if err != nil {
			out.Status = &commonV1.Status{
				Code:    int32(codes.FailedPrecondition),
				Message: err.Error(),
			}

			r.plugin.log.Error("replay from JSON request", "error", err)
			return nil
		}
	default:
		// we have last event ID
		err := replayer.ReplayPartialWorkflowHistoryFromJSONFile(logger.NewSlogAdapter(r.plugin.log), in.GetSavePath(), in.GetLastEventId())
		if err != nil {
			out.Status = &commonV1.Status{
				Code:    int32(codes.FailedPrecondition),
				Message: err.Error(),
			}

			r.plugin.log.Error("replay from JSON request (partial workflow history)", "id", in.GetLastEventId(), "error", err)
			return nil
		}
	}

	out.Status = &commonV1.Status{
		Code: int32(codes.OK),
	}

	r.plugin.log.Debug("replay from JSON request finished successfully")

	return nil
}

func (r *rpc) ReplayWorkflowHistory(in *protoApi.History, out *protoApi.ReplayResponse) error {
	r.plugin.log.Debug("replay from workflow history request",
		"workflow_name", in.GetWorkflowType().GetName(),
	)

	if in.GetHistory() == nil || in.GetWorkflowType().GetName() == "" {
		out.Status = &commonV1.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: "workflow_name and/or history should not be empty",
		}

		r.plugin.log.Error("workflow_name and/or history should not be empty")
		return nil
	}

	if r.plugin.getWfDef() == nil {
		out.Status = &commonV1.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: "workflow definition is not initialized, retry in a second",
		}

		return nil
	}

	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(r.plugin.getWfDef(), workflow.RegisterOptions{
		Name:                          in.GetWorkflowType().GetName(),
		DisableAlreadyRegisteredCheck: false,
	})

	err := replayer.ReplayWorkflowHistory(logger.NewSlogAdapter(r.plugin.log), in.GetHistory())
	if err != nil {
		out.Status = &commonV1.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: err.Error(),
		}

		r.plugin.log.Error("replay workflow history", "error", err)
		return nil
	}

	out.Status = &commonV1.Status{
		Code: int32(codes.OK),
	}

	r.plugin.log.Debug("replay workflow request finished successfully")

	return nil
}

func (r *rpc) UpdateAPIKey(in *string, out *bool) error {
	if in != nil && *in != "" {
		r.plugin.apiKey.Store(in)
		*out = true
		return nil
	}

	*out = false
	return nil
}
