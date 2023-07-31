package roadrunner_temporal //nolint:revive,stylecheck

import (
	"context"
	"os"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	v1Proto "github.com/golang/protobuf/proto" //nolint:staticcheck,nolintlint
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
		return nil
	}

	r.plugin.log.Debug("replay workflow request",
		zap.String("run_id", in.GetWorkflowExecution().GetRunId()),
		zap.String("workflow_id", in.GetWorkflowExecution().GetWorkflowId()),
		zap.String("workflow_name", in.GetWorkflowType().GetName()))

	if in.GetWorkflowExecution().GetRunId() == "" || in.GetWorkflowExecution().GetWorkflowId() == "" || in.GetWorkflowType().GetName() == "" {
		return errors.Str("run_id, workflow_id or workflow_name should not be empty")
	}

	r.plugin.mu.Lock()
	defer r.plugin.mu.Unlock()
	if r.plugin.rrWorkflowDef == nil {
		return errors.Str("workflow defenition is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	iter := r.plugin.client.GetWorkflowHistory(ctx, in.GetWorkflowExecution().GetWorkflowId(), in.GetWorkflowExecution().GetRunId(), false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)

	var hist history.History
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			return err
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
		r.plugin.log.Error("replay error", zap.Error(err))
		return err
	}

	return nil
}

func (r *rpc) DownloadWorkflowHistory(in *protoApi.ReplayRequest, out *protoApi.ReplayResponse) error {
	r.plugin.log.Debug("replay workflow request",
		zap.String("run_id", in.GetWorkflowExecution().GetRunId()),
		zap.String("workflow_id", in.GetWorkflowExecution().GetWorkflowId()),
		zap.String("save_path", in.GetSavePath()))

	if in.GetWorkflowExecution() == nil || in.GetWorkflowType() == nil || in.GetSavePath() == "" {
		return errors.Str("run_id, workflow_id or save_path should not be empty")
	}

	r.plugin.log.Debug("replay workflow request",
		zap.String("run_id", in.GetWorkflowExecution().GetRunId()),
		zap.String("workflow_id", in.GetWorkflowExecution().GetWorkflowId()),
		zap.String("save_path", in.GetSavePath()))

	if in.GetWorkflowExecution().GetRunId() == "" || in.GetWorkflowExecution().GetWorkflowId() == "" || in.GetWorkflowType().GetName() == "" {
		return errors.Str("run_id, workflow_id or workflow_name should not be empty")
	}

	file, err := os.Create(in.GetSavePath())
	if err != nil {
		return err
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
			return errn
		}
		hist.Events = append(hist.Events, event)
	}

	marshaler := jsonpb.Marshaler{}
	err = marshaler.Marshal(file, &hist)
	if err != nil {
		return err
	}

	r.plugin.log.Debug("history saved", zap.String("location", in.GetSavePath()))

	return nil
}

func (r *rpc) ReplayFromJSONPB(in *protoApi.ReplayRequest, out *protoApi.ReplayResponse) error {
	r.plugin.log.Debug("replay workflow request",
		zap.String("workflow_name", in.GetWorkflowType().GetName()),
		zap.String("save_path", in.GetSavePath()))

	if in.GetWorkflowType() == nil || in.GetSavePath() == "" {
		return errors.Str("workflow_name and save_path should not be empty")
	}

	r.plugin.log.Debug("replay workflow request",
		zap.String("workflow_name", in.GetWorkflowType().GetName()),
		zap.String("save_path", in.GetSavePath()))

	if in.GetWorkflowType().GetName() == "" {
		return errors.Str("workflow_name should not be empty")
	}

	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(r.plugin.rrWorkflowDef, workflow.RegisterOptions{
		Name:                          in.GetWorkflowType().GetName(),
		DisableAlreadyRegisteredCheck: false,
	})

	err := replayer.ReplayWorkflowHistoryFromJSONFile(logger.NewZapAdapter(r.plugin.log), in.GetSavePath())
	if err != nil {
		r.plugin.log.Error("replay error", zap.Error(err))
		return err
	}

	return nil
}
