package aggregatedpool

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/roadrunner-server/api/v2/payload"
	"github.com/roadrunner-server/api/v2/pool"
	"github.com/roadrunner-server/errors"
	"github.com/temporalio/roadrunner-temporal/internal"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/workflow"
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
	_ = wp.mq.PushCommand(
		internal.CancelWorkflow{RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID},
		nil,
		wp.header,
	)
}

// schedule the signal processing
func (wp *Workflow) handleSignal(name string, input *commonpb.Payloads, header *commonpb.Header) error {
	_ = wp.mq.PushCommand(
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
		return nil, errors.E(op, bindings.ConvertFailureToError(result.Failure, wp.env.GetDataConverter()))
	}

	return result.Payloads, nil
}

// Workflow incoming command
func (wp *Workflow) handleMessage(msg *internal.Message, actDef *Activity, dc converter.DataConverter) error {
	const op = errors.Op("handleMessage")

	switch command := msg.Command.(type) {
	case *internal.ExecuteActivity:
		params := command.ActivityParams(wp.env, msg.Payloads)
		activityID := wp.env.ExecuteActivity(params, wp.createCallback(msg.ID))

		wp.canceller.Register(msg.ID, func() error {
			wp.env.RequestCancelActivity(activityID)
			return nil
		})

	case *internal.ExecuteLocalActivity:
		/*
			// LocalActivityClient for requesting local activity execution
			LocalActivityClient interface {
				ExecuteLocalActivity(params ExecuteLocalActivityParams, callback LocalActivityResultHandler) LocalActivityID

				RequestCancelLocalActivity(activityID LocalActivityID)
				}
		*/
		params := command.LocalActivityParams(wp.env, actDef.ExecuteLA, dc, msg.Payloads)
		activityID := wp.env.ExecuteLocalActivity(params, wp.createLocalActivityCallback(msg.ID))
		wp.canceller.Register(msg.ID, func() error {
			wp.env.RequestCancelLocalActivity(activityID)
			return nil
		})

	case *internal.ExecuteChildWorkflow:
		params := command.WorkflowParams(wp.env, msg.Payloads)

		// always use deterministic id
		if params.WorkflowID == "" {
			nextID := atomic.AddUint64(&wp.seqID, 1)
			params.WorkflowID = wp.env.WorkflowInfo().WorkflowExecution.RunID + "_" + strconv.Itoa(int(nextID))
		}

		wp.env.ExecuteChildWorkflow(params, wp.createCallback(msg.ID), func(r bindings.WorkflowExecution, e error) {
			wp.ids.Push(msg.ID, r, e)
		})

		wp.canceller.Register(msg.ID, func() error {
			wp.env.RequestCancelChildWorkflow(params.Namespace, params.WorkflowID)
			return nil
		})

	case *internal.GetChildWorkflowExecution:
		wp.ids.Listen(command.ID, func(w bindings.WorkflowExecution, err error) {
			cl := wp.createCallback(msg.ID)

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
		timerID := wp.env.NewTimer(command.ToDuration(), wp.createCallback(msg.ID))
		wp.canceller.Register(msg.ID, func() error {
			if timerID != nil {
				wp.env.RequestCancelTimer(*timerID)
			}
			return nil
		})

	case *internal.GetVersion:
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
		wp.env.SideEffect(
			func() (*commonpb.Payloads, error) {
				return msg.Payloads, nil
			},
			wp.createContinuableCallback(msg.ID),
		)

	case *internal.CompleteWorkflow:
		result, _ := wp.env.GetDataConverter().ToPayloads(completed)
		wp.mq.PushResponse(msg.ID, result)

		if msg.Failure == nil {
			wp.env.Complete(msg.Payloads, nil)
			return nil
		}

		wp.env.Complete(nil, bindings.ConvertFailureToError(msg.Failure, wp.env.GetDataConverter()))

	case *internal.ContinueAsNew:
		result, _ := wp.env.GetDataConverter().ToPayloads(completed)
		wp.mq.PushResponse(msg.ID, result)

		wp.env.Complete(nil, &workflow.ContinueAsNewError{
			WorkflowType:             &bindings.WorkflowType{Name: command.Name},
			Input:                    msg.Payloads,
			Header:                   msg.Header,
			TaskQueueName:            command.Options.TaskQueueName,
			WorkflowExecutionTimeout: command.Options.WorkflowExecutionTimeout,
			WorkflowRunTimeout:       command.Options.WorkflowRunTimeout,
			WorkflowTaskTimeout:      command.Options.WorkflowTaskTimeout,
		})

	case *internal.SignalExternalWorkflow:
		wp.env.SignalExternalWorkflow(
			command.Namespace,
			command.WorkflowID,
			command.RunID,
			command.Signal,
			msg.Payloads,
			nil,
			msg.Header,
			command.ChildWorkflowOnly,
			wp.createCallback(msg.ID),
		)

	case *internal.CancelExternalWorkflow:
		wp.env.RequestCancelExternalWorkflow(command.Namespace, command.WorkflowID, command.RunID, wp.createCallback(msg.ID))

	case *internal.Cancel:
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
		// do not wrap error to pass it directly to Temporal
		return bindings.ConvertFailureToError(msg.Failure, wp.env.GetDataConverter())

	default:
		return errors.E(op, errors.Str("undefined command"))
	}

	return nil
}

func (wp *Workflow) createLocalActivityCallback(id uint64) bindings.LocalActivityResultHandler {
	callback := func(lar *bindings.LocalActivityResultWrapper) {
		wp.canceller.Discard(id)

		if lar.Err != nil {
			wp.mq.PushError(id, bindings.ConvertErrorToFailure(lar.Err, wp.env.GetDataConverter()))
			return
		}

		wp.mq.PushResponse(id, lar.Result)
	}

	return func(lar *bindings.LocalActivityResultWrapper) {
		// timer cancel callback can happen inside the loop
		if atomic.LoadUint32(&wp.inLoop) == 1 {
			callback(lar)

			if lar.Err != nil {
				panic(lar.Err)
			}
			return
		}

		if lar.Err != nil {
			wp.mq.PushError(id, bindings.ConvertErrorToFailure(lar.Err, wp.env.GetDataConverter()))
			return
		}

		wp.callbacks = append(wp.callbacks, func() error {
			callback(lar)
			return lar.Err
		})
	}
}

func (wp *Workflow) createCallback(id uint64) bindings.ResultHandler {
	callback := func(result *commonpb.Payloads, err error) error {
		wp.canceller.Discard(id)

		if err != nil {
			wp.mq.PushError(id, bindings.ConvertErrorToFailure(err, wp.env.GetDataConverter()))
			return nil
		}

		// fetch original payload
		wp.mq.PushResponse(id, result)
		return nil
	}

	return func(result *commonpb.Payloads, err error) {
		// timer cancel callback can happen inside the loop
		if atomic.LoadUint32(&wp.inLoop) == 1 {
			errC := callback(result, err)
			if errC != nil {
				panic(errC)
			}

			return
		}

		wp.callbacks = append(wp.callbacks, func() error {
			return callback(result, err)
		})
	}
}

// callback to be called inside the queue processing, adds new messages at the end of the queue
func (wp *Workflow) createContinuableCallback(id uint64) bindings.ResultHandler {
	callback := func(result *commonpb.Payloads, err error) {
		wp.canceller.Discard(id)

		if err != nil {
			wp.mq.PushError(id, bindings.ConvertErrorToFailure(err, wp.env.GetDataConverter()))
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
	const op = errors.Op("flush queue")

	if len(wp.mq.Queue) == 0 {
		return nil
	}

	if wp.mh != nil {
		wp.mh.Gauge(RrWorkflowsMetricName).Update(float64(wp.pool.(pool.Queuer).QueueSize()))
		defer wp.mh.Gauge(RrWorkflowsMetricName).Update(float64(wp.pool.(pool.Queuer).QueueSize()))
	}

	pld := &payload.Payload{}
	err := wp.codec.Encode(wp.getContext(), pld, wp.mq.Queue...)
	if err != nil {
		return err
	}

	resp, err := wp.pool.Exec(pld)
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
func (wp *Workflow) runCommand(cmd interface{}, payloads *commonpb.Payloads, header *commonpb.Header) (*internal.Message, error) {
	const op = errors.Op("workflow_process_runcommand")
	_, msg := wp.mq.AllocateMessage(cmd, payloads, header)

	if wp.mh != nil {
		wp.mh.Gauge(RrMetricName).Update(float64(wp.pool.(pool.Queuer).QueueSize()))
		defer wp.mh.Gauge(RrMetricName).Update(float64(wp.pool.(pool.Queuer).QueueSize()))
	}

	pld := &payload.Payload{}
	err := wp.codec.Encode(wp.getContext(), pld, &msg)
	if err != nil {
		return nil, err
	}

	resp, err := wp.pool.Exec(pld)
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
