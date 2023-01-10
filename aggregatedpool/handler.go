package aggregatedpool

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v3/payload"
	"github.com/temporalio/roadrunner-temporal/v3/internal"
	commonpb "go.temporal.io/api/common/v1"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
)

const (
	completed string = "completed"
)

// execution context.
func (wp *Workflow) getContext() *internal.Context {
	return &internal.Context{
		TaskQueue: wp.env.WorkflowInfo().TaskQueueName,
		TickTime:  wp.env.Now().Format(time.RFC3339),
		Replay:    wp.env.IsReplaying(),
	}
}

// schedule cancel command
func (wp *Workflow) handleCancel() {
	wp.mq.PushCommand(
		internal.CancelWorkflow{RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID},
		nil,
		wp.header,
	)
}

// schedule the signal processing
func (wp *Workflow) handleSignal(name string, input *commonpb.Payloads, header *commonpb.Header) error {
	wp.mq.PushCommand(
		internal.InvokeSignal{
			RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID,
			Name:  name,
		},
		input,
		header,
	)

	return nil
}

// Handle query in blocking mode.
func (wp *Workflow) handleQuery(queryType string, queryArgs *commonpb.Payloads, header *commonpb.Header) (*commonpb.Payloads, error) {
	const op = errors.Op("workflow_process_handle_query")
	result, err := wp.runCommand(internal.InvokeQuery{
		RunID: wp.runID,
		Name:  queryType,
	}, queryArgs, header)

	if err != nil {
		return nil, errors.E(op, err)
	}

	if result.Failure != nil {
		return nil, errors.E(op, temporal.GetDefaultFailureConverter().FailureToError(result.Failure))
	}

	return result.Payloads, nil
}

// Workflow incoming command
func (wp *Workflow) handleMessage(msg *internal.Message) error {
	const op = errors.Op("handleMessage")

	switch command := msg.Command.(type) {
	case *internal.ExecuteActivity:
		wp.log.Debug("activity request", zap.Uint64("ID", msg.ID))
		params := command.ActivityParams(wp.env, msg.Payloads, msg.Header)
		activityID := wp.env.ExecuteActivity(params, wp.createCallback(msg.ID, "activity"))

		wp.canceller.Register(msg.ID, func() error {
			wp.log.Debug("registering activity canceller", zap.String("activityID", activityID.String()))
			wp.env.RequestCancelActivity(activityID)
			return nil
		})

	case *internal.ExecuteLocalActivity:
		wp.log.Debug("local activity request", zap.Uint64("ID", msg.ID))
		params := command.LocalActivityParams(wp.env, NewLocalActivityFn(msg.Header, wp.codec, wp.pool, wp.log).execute, msg.Payloads, msg.Header)
		activityID := wp.env.ExecuteLocalActivity(params, wp.createLocalActivityCallback(msg.ID))
		wp.canceller.Register(msg.ID, func() error {
			wp.log.Debug("registering local activity canceller", zap.String("activityID", activityID.String()))
			wp.env.RequestCancelLocalActivity(activityID)
			return nil
		})

	case *internal.ExecuteChildWorkflow:
		wp.log.Debug("execute child workflow request", zap.Uint64("ID", msg.ID))
		params := command.WorkflowParams(wp.env, msg.Payloads, msg.Header)

		// always use deterministic id
		if params.WorkflowID == "" {
			nextID := atomic.AddUint64(&wp.seqID, 1)
			params.WorkflowID = wp.env.WorkflowInfo().WorkflowExecution.RunID + "_" + strconv.Itoa(int(nextID))
		}

		wp.env.ExecuteChildWorkflow(params, wp.createCallback(msg.ID, "ExecuteChildWorkflow"), func(r bindings.WorkflowExecution, e error) {
			wp.ids.Push(msg.ID, r, e)
		})

		wp.canceller.Register(msg.ID, func() error {
			wp.env.RequestCancelChildWorkflow(params.Namespace, params.WorkflowID)
			return nil
		})

	case *internal.GetChildWorkflowExecution:
		wp.log.Debug("get child workflow execution request", zap.Uint64("ID", msg.ID))
		wp.ids.Listen(command.ID, func(w bindings.WorkflowExecution, err error) {
			cl := wp.createCallback(msg.ID, "GetChildWorkflow")

			if err != nil {
				cl(nil, err)
				return
			}

			p, er := wp.env.GetDataConverter().ToPayloads(w)
			if er != nil {
				panic(er)
			}

			cl(p, err)
		})

	case *internal.NewTimer:
		wp.log.Debug("timer request", zap.Uint64("ID", msg.ID))
		timerID := wp.env.NewTimer(command.ToDuration(), wp.createCallback(msg.ID, "NewTimer"))
		wp.canceller.Register(msg.ID, func() error {
			if timerID != nil {
				wp.log.Debug("cancel timer request", zap.String("timerID", timerID.String()))
				wp.env.RequestCancelTimer(*timerID)
			}
			return nil
		})

	case *internal.GetVersion:
		wp.log.Debug("get version request", zap.Uint64("ID", msg.ID))
		version := wp.env.GetVersion(
			command.ChangeID,
			workflow.Version(command.MinSupported),
			workflow.Version(command.MaxSupported),
		)

		result, err := wp.env.GetDataConverter().ToPayloads(version)
		if err != nil {
			return errors.E(op, err)
		}

		wp.mq.PushResponse(msg.ID, result)
		err = wp.flushQueue()
		if err != nil {
			return errors.E(op, err)
		}

	case *internal.SideEffect:
		wp.log.Debug("side-effect request", zap.Uint64("ID", msg.ID))
		wp.env.SideEffect(
			func() (*commonpb.Payloads, error) {
				return msg.Payloads, nil
			},
			wp.createContinuableCallback(msg.ID, "SideEffect"),
		)

	case *internal.CompleteWorkflow:
		wp.log.Debug("complete workflow request", zap.Uint64("ID", msg.ID))
		result, _ := wp.env.GetDataConverter().ToPayloads(completed)
		wp.mq.PushResponse(msg.ID, result)

		if msg.Failure == nil {
			wp.env.Complete(msg.Payloads, nil)
			return nil
		}

		wp.env.Complete(nil, temporal.GetDefaultFailureConverter().FailureToError(msg.Failure))

	case *internal.ContinueAsNew:
		wp.log.Debug("continue-as-new request", zap.Uint64("ID", msg.ID), zap.String("name", command.Name))
		result, _ := wp.env.GetDataConverter().ToPayloads(completed)
		wp.mq.PushResponse(msg.ID, result)

		wp.env.Complete(nil, &workflow.ContinueAsNewError{
			WorkflowType: &bindings.WorkflowType{
				Name: command.Name,
			},
			Input:               msg.Payloads,
			Header:              msg.Header,
			TaskQueueName:       command.Options.TaskQueueName,
			WorkflowRunTimeout:  command.Options.WorkflowRunTimeout,
			WorkflowTaskTimeout: command.Options.WorkflowTaskTimeout,
		})

	case *internal.UpsertWorkflowSearchAttributes:
		wp.log.Debug("upsert search attributes request", zap.Uint64("ID", msg.ID))
		err := wp.env.UpsertSearchAttributes(command.SearchAttributes)
		if err != nil {
			return errors.E(op, err)
		}

	case *internal.SignalExternalWorkflow:
		wp.log.Debug("signal external workflow request", zap.Uint64("ID", msg.ID))
		wp.env.SignalExternalWorkflow(
			command.Namespace,
			command.WorkflowID,
			command.RunID,
			command.Signal,
			msg.Payloads,
			nil,
			msg.Header,
			command.ChildWorkflowOnly,
			wp.createCallback(msg.ID, "SignalExternalWorkflow"),
		)

	case *internal.CancelExternalWorkflow:
		wp.log.Debug("cancel external workflow request", zap.Uint64("ID", msg.ID))
		wp.env.RequestCancelExternalWorkflow(command.Namespace, command.WorkflowID, command.RunID, wp.createCallback(msg.ID, "CancelExternalWorkflow"))

	case *internal.Cancel:
		wp.log.Debug("cancel request", zap.Uint64("ID", msg.ID))
		err := wp.canceller.Cancel(command.CommandIDs...)
		if err != nil {
			return errors.E(op, err)
		}

		result, _ := wp.env.GetDataConverter().ToPayloads(completed)
		wp.mq.PushResponse(msg.ID, result)

		err = wp.flushQueue()
		if err != nil {
			return errors.E(op, err)
		}

	case *internal.Panic:
		wp.log.Debug("panic", zap.String("failure", msg.Failure.String()))
		// do not wrap error to pass it directly to Temporal
		return temporal.GetDefaultFailureConverter().FailureToError(msg.Failure)

	default:
		return errors.E(op, errors.Str("undefined command"))
	}

	return nil
}

func (wp *Workflow) createLocalActivityCallback(id uint64) bindings.LocalActivityResultHandler {
	callback := func(lar *bindings.LocalActivityResultWrapper) {
		wp.log.Debug("executing local activity callback", zap.Uint64("ID", id))
		wp.canceller.Discard(id)

		if lar.Err != nil {
			wp.log.Debug("error", zap.Error(lar.Err), zap.Int32("attempt", lar.Attempt), zap.Duration("backoff", lar.Backoff))
			wp.mq.PushError(id, temporal.GetDefaultFailureConverter().ErrorToFailure(lar.Err))
			return
		}

		wp.log.Debug("pushing local activity response", zap.Uint64("ID", id))
		wp.mq.PushResponse(id, lar.Result)
	}

	return func(lar *bindings.LocalActivityResultWrapper) {
		// timer cancel callback can happen inside the loop
		if atomic.LoadUint32(&wp.inLoop) == 1 {
			wp.log.Debug("calling local activity callback IN LOOP", zap.Uint64("ID", id))
			callback(lar)
			return
		}

		wp.callbacks = append(wp.callbacks, func() error {
			wp.log.Debug("appending local activity callback", zap.Uint64("ID", id))
			callback(lar)
			return nil
		})
	}
}

func (wp *Workflow) createCallback(id uint64, t string) bindings.ResultHandler {
	callback := func(result *commonpb.Payloads, err error) {
		wp.log.Debug("executing callback", zap.Uint64("ID", id), zap.String("type", t))
		wp.canceller.Discard(id)

		if err != nil {
			wp.log.Debug("error", zap.Error(err), zap.String("type", t))
			wp.mq.PushError(id, temporal.GetDefaultFailureConverter().ErrorToFailure(err))
			return
		}

		wp.log.Debug("pushing response", zap.Uint64("ID", id), zap.String("type", t))
		// fetch original payload
		wp.mq.PushResponse(id, result)
	}

	return func(result *commonpb.Payloads, err error) {
		// timer cancel callback can happen inside the loop
		if atomic.LoadUint32(&wp.inLoop) == 1 {
			wp.log.Debug("calling callback IN LOOP", zap.Uint64("ID", id), zap.String("type", t))
			callback(result, err)
			return
		}

		wp.callbacks = append(wp.callbacks, func() error {
			wp.log.Debug("appending callback", zap.Uint64("ID", id), zap.String("type", t))
			callback(result, err)
			return nil
		})
	}
}

// callback to be called inside the queue processing, adds new messages at the end of the queue
func (wp *Workflow) createContinuableCallback(id uint64, t string) bindings.ResultHandler {
	callback := func(result *commonpb.Payloads, err error) {
		wp.log.Debug("executing continuable callback", zap.Uint64("ID", id), zap.String("type", t))
		wp.canceller.Discard(id)

		if err != nil {
			wp.mq.PushError(id, temporal.GetDefaultFailureConverter().ErrorToFailure(err))
			return
		}

		wp.mq.PushResponse(id, result)
		err = wp.flushQueue()
		if err != nil {
			panic(err)
		}
	}

	return func(result *commonpb.Payloads, err error) {
		callback(result, err)
	}
}

// Exchange messages between host and pool processes and add new commands to the queue.
func (wp *Workflow) flushQueue() error {
	const op = errors.Op("flush_queue")

	if len(wp.mq.Messages()) == 0 {
		return nil
	}

	if wp.mh != nil {
		wp.mh.Gauge(RrWorkflowsMetricName).Update(float64(wp.pool.QueueSize()))
		defer wp.mh.Gauge(RrWorkflowsMetricName).Update(float64(wp.pool.QueueSize()))
	}

	// todo(rustatian) to sync.Pool
	pld := &payload.Payload{}
	err := wp.codec.Encode(wp.getContext(), pld, wp.mq.Messages()...)
	if err != nil {
		return err
	}

	resp, err := wp.pool.Exec(context.Background(), pld)
	if err != nil {
		return err
	}

	msgs := make([]*internal.Message, 0, 2)
	err = wp.codec.Decode(resp, &msgs)
	if err != nil {
		return err
	}
	wp.mq.Flush()

	if err != nil {
		return errors.E(op, err)
	}

	wp.pipeline = append(wp.pipeline, msgs...)

	return nil
}

// Run single command and return single result.
func (wp *Workflow) runCommand(cmd any, payloads *commonpb.Payloads, header *commonpb.Header) (*internal.Message, error) {
	const op = errors.Op("workflow_process_runcommand")
	msg := wp.mq.AllocateMessage(cmd, payloads, header)

	if wp.mh != nil {
		wp.mh.Gauge(RrMetricName).Update(float64(wp.pool.QueueSize()))
		defer wp.mh.Gauge(RrMetricName).Update(float64(wp.pool.QueueSize()))
	}

	pld := &payload.Payload{}
	err := wp.codec.Encode(wp.getContext(), pld, msg)
	if err != nil {
		return nil, err
	}

	// todo(rustatian): do we need a timeout here??
	resp, err := wp.pool.Exec(context.Background(), pld)
	if err != nil {
		return nil, err
	}

	msgs := make([]*internal.Message, 0, 2)
	err = wp.codec.Decode(resp, &msgs)
	if err != nil {
		return nil, err
	}

	if len(msgs) != 1 {
		return nil, errors.E(op, errors.Str("unexpected pool response"))
	}

	return msgs[0], nil
}
