package rrtemporal

import (
	"context"
	"os"
	"time"

	"github.com/roadrunner-server/api/v4/build/common/v1"
	protoApi "github.com/roadrunner-server/api/v4/build/temporal/v1"
	"github.com/roadrunner-server/errors"
	"github.com/temporalio/roadrunner-temporal/v4/internal/logger"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
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
	r.plugin.mu.RLock()
	ctx, err := r.plugin.temporal.rrActivityDef.GetActivityContext(in.TaskToken)
	if err != nil {
		r.plugin.mu.RUnlock()
		return err
	}
	r.plugin.mu.RUnlock()

	activity.RecordHeartbeat(ctx, details)

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
		zap.String("run_id", in.GetWorkflowExecution().GetRunId()),
		zap.String("workflow_id", in.GetWorkflowExecution().GetWorkflowId()),
		zap.String("workflow_name", in.GetWorkflowType().GetName()))

	if in.GetWorkflowExecution() == nil || in.GetWorkflowType() == nil {
		out.Status = &common.Status{
			Code:    int32(codes.InvalidArgument),
			Message: "run_id, workflow_id or workflow_name should not be empty",
		}

		r.plugin.log.Error("replay workflow request", zap.String("error", "run_id, workflow_id or workflow_name should not be empty"))
		return nil
	}

	if in.GetWorkflowExecution().GetRunId() == "" || in.GetWorkflowExecution().GetWorkflowId() == "" || in.GetWorkflowType().GetName() == "" {
		out.Status = &common.Status{
			Code:    int32(codes.InvalidArgument),
			Message: "run_id, workflow_id or workflow_name should not be empty",
		}

		r.plugin.log.Error("replay workflow request", zap.String("error", "run_id, workflow_id or workflow_name should not be empty"))
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	iter := r.plugin.temporal.client.GetWorkflowHistory(ctx, in.GetWorkflowExecution().GetWorkflowId(), in.GetWorkflowExecution().GetRunId(), false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)

	var hist history.History
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			out.Status = &common.Status{
				Code:    int32(codes.Internal),
				Message: err.Error(),
			}

			r.plugin.log.Error("history iteration error", zap.Error(err))
			return nil
		}
		hist.Events = append(hist.Events, event)
	}

	if r.plugin.getWfDef() == nil {
		out.Status = &common.Status{
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

	err := replayer.ReplayWorkflowHistory(logger.NewZapAdapter(r.plugin.log), &hist)
	if err != nil {
		out.Status = &common.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: err.Error(),
		}

		r.plugin.log.Error("replay error", zap.Error(err))
		return nil
	}

	out.Status = &common.Status{
		Code: int32(codes.OK),
	}

	r.plugin.log.Debug("replay workflow request finished successfully")

	return nil
}

func (r *rpc) DownloadWorkflowHistory(in *protoApi.ReplayRequest, out *protoApi.ReplayResponse) error {
	r.plugin.log.Debug("replay workflow request",
		zap.String("run_id", in.GetWorkflowExecution().GetRunId()),
		zap.String("workflow_id", in.GetWorkflowExecution().GetWorkflowId()),
		zap.String("save_path", in.GetSavePath()))

	if in.GetWorkflowExecution() == nil || in.GetWorkflowType() == nil || in.GetSavePath() == "" {
		out.Status = &common.Status{
			Code:    int32(codes.InvalidArgument),
			Message: "run_id, workflow_id or save_path should not be empty",
		}

		return nil
	}

	if in.GetWorkflowExecution().GetRunId() == "" || in.GetWorkflowExecution().GetWorkflowId() == "" || in.GetWorkflowType().GetName() == "" {
		out.Status = &common.Status{
			Code:    int32(codes.InvalidArgument),
			Message: "run_id, workflow_id or save_path should not be empty",
		}

		r.plugin.log.Error("replay workflow request", zap.String("error", "run_id, workflow_id or save_path should not be empty"))
		return nil
	}

	file, err := os.Create(in.GetSavePath())
	if err != nil {
		out.Status = &common.Status{
			Code:    int32(codes.Internal),
			Message: err.Error(),
		}

		r.plugin.log.Error("failed to create the file", zap.Error(err))
		return nil
	}

	defer func() {
		err = file.Close()
		if err != nil {
			r.plugin.log.Error("failed to close the file", zap.Error(err))
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	iter := r.plugin.temporal.client.GetWorkflowHistory(ctx, in.GetWorkflowExecution().GetWorkflowId(), in.GetWorkflowExecution().GetRunId(), false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)

	var hist history.History

	for iter.HasNext() {
		event, errn := iter.Next()
		if errn != nil {
			out.Status = &common.Status{
				Code:    int32(codes.Internal),
				Message: errn.Error(),
			}

			r.plugin.log.Error("history iteration error", zap.Error(errn))
			return nil
		}

		hist.Events = append(hist.Events, event)
	}

	data, err := protojson.Marshal(&hist)
	if err != nil {
		out.Status = &common.Status{
			Code:    int32(codes.Internal),
			Message: err.Error(),
		}

		r.plugin.log.Error("history marshal error", zap.Error(err))
		return nil
	}

	_, err = file.Write(data)
	if err != nil {
		out.Status = &common.Status{
			Code:    int32(codes.Internal),
			Message: err.Error(),
		}

		r.plugin.log.Error("history marshal error", zap.Error(err))
		return nil
	}

	out.Status = &common.Status{
		Code: int32(codes.OK),
	}

	r.plugin.log.Debug("history saved", zap.String("location", in.GetSavePath()))

	return nil
}

func (r *rpc) ReplayFromJSON(in *protoApi.ReplayRequest, out *protoApi.ReplayResponse) error {
	r.plugin.log.Debug("replay from JSON request",
		zap.String("workflow_name", in.GetWorkflowType().GetName()),
		zap.String("save_path", in.GetSavePath()),
		zap.Int64("last_event_id", in.GetLastEventId()),
	)

	if in.GetWorkflowType() == nil || in.GetSavePath() == "" {
		out.Status = &common.Status{
			Code:    int32(codes.InvalidArgument),
			Message: "workflow_name and save_path should not be empty",
		}

		r.plugin.log.Error("replay from JSON request", zap.String("error", "workflow_name and save_path should not be empty"))
		return nil
	}

	if in.GetWorkflowType().GetName() == "" {
		out.Status = &common.Status{
			Code:    int32(codes.InvalidArgument),
			Message: "workflow_name should not be empty",
		}

		r.plugin.log.Error("replay from JSON request", zap.String("error", "workflow_name should not be empty"))
		return nil
	}

	if r.plugin.getWfDef() == nil {
		out.Status = &common.Status{
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
		err := replayer.ReplayWorkflowHistoryFromJSONFile(logger.NewZapAdapter(r.plugin.log), in.GetSavePath())
		if err != nil {
			out.Status = &common.Status{
				Code:    int32(codes.FailedPrecondition),
				Message: err.Error(),
			}

			r.plugin.log.Error("replay from JSON request", zap.Error(err))
			return nil
		}
	default:
		// we have last event ID
		err := replayer.ReplayPartialWorkflowHistoryFromJSONFile(logger.NewZapAdapter(r.plugin.log), in.GetSavePath(), in.GetLastEventId())
		if err != nil {
			out.Status = &common.Status{
				Code:    int32(codes.FailedPrecondition),
				Message: err.Error(),
			}

			r.plugin.log.Error("replay from JSON request (partial workflow history)", zap.Int64("id", in.GetLastEventId()), zap.Error(err))
			return nil
		}
	}

	out.Status = &common.Status{
		Code: int32(codes.OK),
	}

	r.plugin.log.Debug("replay from JSON request finished successfully")

	return nil
}

func (r *rpc) ReplayWorkflowHistory(in *protoApi.History, out *protoApi.ReplayResponse) error {
	r.plugin.log.Debug("replay from workflow history request",
		zap.String("workflow_name", in.GetWorkflowType().GetName()),
	)

	if in.GetHistory() == nil || in.GetWorkflowType().GetName() == "" {
		out.Status = &common.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: "workflow_name and/or history should not be empty",
		}

		r.plugin.log.Error("workflow_name and/or history should not be empty")
		return nil
	}

	if r.plugin.getWfDef() == nil {
		out.Status = &common.Status{
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

	err := replayer.ReplayWorkflowHistory(logger.NewZapAdapter(r.plugin.log), in.GetHistory())
	if err != nil {
		out.Status = &common.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: err.Error(),
		}

		r.plugin.log.Error("replay workflow history", zap.Error(err))
		return nil
	}

	out.Status = &common.Status{
		Code: int32(codes.OK),
	}

	r.plugin.log.Debug("replay workflow request finished successfully")

	return nil
}
