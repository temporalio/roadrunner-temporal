package roadrunner_temporal //nolint:revive,stylecheck

import (
	"context"
	"os"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	v1Proto "github.com/golang/protobuf/proto" //nolint:staticcheck,nolintlint
	"github.com/roadrunner-server/api/v4/build/common/v1"
	protoApi "github.com/roadrunner-server/api/v4/build/temporal/v1"
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
		if err := proto.Unmarshal(in.Details, v1Proto.MessageV2(details)); err != nil {
			return err
		}
	}

	// find running activity
	r.plugin.mu.RLock()
	ctx, err := r.plugin.rrActivityDef.GetActivityContext(in.TaskToken)
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
	for k := range r.plugin.activities {
		*out = append(*out, k)
	}
	return nil
}

func (r *rpc) GetWorkflowNames(_ bool, out *[]string) error {
	r.plugin.mu.RLock()
	defer r.plugin.mu.RUnlock()

	for k := range r.plugin.workflows {
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
		*out.Status = common.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: "run_id, workflow_id or workflow_name should not be empty",
		}

		r.plugin.log.Error("replay workflow request", zap.String("error", "run_id, workflow_id or workflow_name should not be empty"))
		return nil
	}

	r.plugin.log.Debug("replay workflow request",
		zap.String("run_id", in.GetWorkflowExecution().GetRunId()),
		zap.String("workflow_id", in.GetWorkflowExecution().GetWorkflowId()),
		zap.String("workflow_name", in.GetWorkflowType().GetName()))

	if in.GetWorkflowExecution().GetRunId() == "" || in.GetWorkflowExecution().GetWorkflowId() == "" || in.GetWorkflowType().GetName() == "" {
		*out.Status = common.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: "run_id, workflow_id or workflow_name should not be empty",
		}

		r.plugin.log.Error("replay workflow request", zap.String("error", "run_id, workflow_id or workflow_name should not be empty"))
	}

	if r.plugin.rrWorkflowDef == nil {
		*out.Status = common.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: "workflow definition is not initialized",
		}

		r.plugin.log.Error("replay workflow request", zap.String("error", "workflow definition is not initialized"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	iter := r.plugin.client.GetWorkflowHistory(ctx, in.GetWorkflowExecution().GetWorkflowId(), in.GetWorkflowExecution().GetRunId(), false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)

	var hist history.History
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			*out.Status = common.Status{
				Code:    int32(codes.Internal),
				Message: err.Error(),
			}

			r.plugin.log.Error("history iteration error", zap.Error(err))
			return nil
		}
		hist.Events = append(hist.Events, event)
	}

	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(r.plugin.rrWorkflowDef, workflow.RegisterOptions{
		Name:                          in.GetWorkflowType().GetName(),
		DisableAlreadyRegisteredCheck: false,
	})

	err := replayer.ReplayWorkflowHistory(logger.NewZapAdapter(r.plugin.log), &hist)
	if err != nil {
		*out.Status = common.Status{
			Code:    int32(codes.Internal),
			Message: err.Error(),
		}

		r.plugin.log.Error("replay error", zap.Error(err))
		return nil
	}

	return nil
}

func (r *rpc) DownloadWorkflowHistory(in *protoApi.ReplayRequest, out *protoApi.ReplayResponse) error {
	r.plugin.log.Debug("replay workflow request",
		zap.String("run_id", in.GetWorkflowExecution().GetRunId()),
		zap.String("workflow_id", in.GetWorkflowExecution().GetWorkflowId()),
		zap.String("save_path", in.GetSavePath()))

	if in.GetWorkflowExecution() == nil || in.GetWorkflowType() == nil || in.GetSavePath() == "" {
		*out.Status = common.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: "run_id, workflow_id or save_path should not be empty",
		}

		return nil
	}

	r.plugin.log.Debug("replay workflow request",
		zap.String("run_id", in.GetWorkflowExecution().GetRunId()),
		zap.String("workflow_id", in.GetWorkflowExecution().GetWorkflowId()),
		zap.String("save_path", in.GetSavePath()))

	if in.GetWorkflowExecution().GetRunId() == "" || in.GetWorkflowExecution().GetWorkflowId() == "" || in.GetWorkflowType().GetName() == "" {
		*out.Status = common.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: "run_id, workflow_id or save_path should not be empty",
		}

		r.plugin.log.Error("replay workflow request", zap.String("error", "run_id, workflow_id or save_path should not be empty"))
		return nil
	}

	file, err := os.Create(in.GetSavePath())
	if err != nil {
		*out.Status = common.Status{
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
	iter := r.plugin.client.GetWorkflowHistory(ctx, in.GetWorkflowExecution().GetWorkflowId(), in.GetWorkflowExecution().GetRunId(), false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)

	var hist history.History

	for iter.HasNext() {
		event, errn := iter.Next()
		if errn != nil {
			*out.Status = common.Status{
				Code:    int32(codes.Internal),
				Message: errn.Error(),
			}

			r.plugin.log.Error("history iteration error", zap.Error(errn))
			return nil
		}

		hist.Events = append(hist.Events, event)
	}

	marshaler := jsonpb.Marshaler{}
	err = marshaler.Marshal(file, &hist)
	if err != nil {
		*out.Status = common.Status{
			Code:    int32(codes.Internal),
			Message: err.Error(),
		}

		r.plugin.log.Error("history marshal error", zap.Error(err))
		return nil
	}

	r.plugin.log.Debug("history saved", zap.String("location", in.GetSavePath()))

	return nil
}

func (r *rpc) ReplayFromJSON(in *protoApi.ReplayRequest, out *protoApi.ReplayResponse) error {
	r.plugin.log.Debug("replay from JSON request",
		zap.String("workflow_name", in.GetWorkflowType().GetName()),
		zap.String("save_path", in.GetSavePath()))

	if in.GetWorkflowType() == nil || in.GetSavePath() == "" {
		*out.Status = common.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: "workflow_name and save_path should not be empty",
		}

		r.plugin.log.Error("replay from JSON request", zap.String("error", "workflow_name and save_path should not be empty"))
		return nil
	}

	r.plugin.log.Debug("replay from JSON request",
		zap.String("workflow_name", in.GetWorkflowType().GetName()),
		zap.String("save_path", in.GetSavePath()))

	if in.GetWorkflowType().GetName() == "" {
		*out.Status = common.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: "workflow_name should not be empty",
		}

		r.plugin.log.Error("replay from JSON request", zap.String("error", "workflow_name should not be empty"))
		return nil
	}

	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(r.plugin.rrWorkflowDef, workflow.RegisterOptions{
		Name:                          in.GetWorkflowType().GetName(),
		DisableAlreadyRegisteredCheck: false,
	})

	switch in.GetLastEventId() {
	// we don't have last event ID
	case 0:
		err := replayer.ReplayWorkflowHistoryFromJSONFile(logger.NewZapAdapter(r.plugin.log), in.GetSavePath())
		if err != nil {
			*out.Status = common.Status{
				Code:    int32(codes.Internal),
				Message: err.Error(),
			}

			r.plugin.log.Error("replay from JSON request", zap.Error(err))
		}
	default:
		// we have last event ID
		err := replayer.ReplayPartialWorkflowHistoryFromJSONFile(logger.NewZapAdapter(r.plugin.log), in.GetSavePath(), in.GetLastEventId())
		if err != nil {
			*out.Status = common.Status{
				Code:    int32(codes.Internal),
				Message: err.Error(),
			}

			r.plugin.log.Error("replay from JSON request (partial workflow history)", zap.Error(err))
		}
	}

	return nil
}
