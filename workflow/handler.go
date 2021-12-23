package workflow

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/spiral/errors"
	bindings "github.com/spiral/sdk-go/internalbindings"
	"github.com/spiral/sdk-go/workflow"
	"github.com/temporalio/roadrunner-temporal/internal"
	commonpb "go.temporal.io/api/common/v1"
)

const completed string = "completed"

// execution context.
func (wp *process) getContext() *internal.Context {
	return &internal.Context{
		TaskQueue: wp.env.WorkflowInfo().TaskQueueName,
		TickTime:  wp.env.Now().Format(time.RFC3339),
		Replay:    wp.env.IsReplaying(),
	}
}

// schedule cancel command
func (wp *process) handleCancel() {
	_ = wp.mq.PushCommand(
		internal.CancelWorkflow{RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID},
		nil,
	)
}

// schedule the signal processing
func (wp *process) handleSignal(name string, input *commonpb.Payloads) error {
	_ = wp.mq.PushCommand(
		internal.InvokeSignal{
			RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID,
			Name:  name,
		},
		input,
	)

	return nil
}

// Handle query in blocking mode.
func (wp *process) handleQuery(queryType string, queryArgs *commonpb.Payloads) (*commonpb.Payloads, error) {
	const op = errors.Op("workflow_process_handle_query")
	result, err := wp.runCommand(internal.InvokeQuery{
		RunID: wp.runID,
		Name:  queryType,
	}, queryArgs)

	if err != nil {
		return nil, errors.E(op, err)
	}

	if result.Failure != nil {
		return nil, errors.E(op, bindings.ConvertFailureToError(result.Failure, wp.env.GetDataConverter()))
	}

	return result.Payloads, nil
}

// process incoming command
func (wp *process) handleMessage(msg *internal.Message) error {
	const op = errors.Op("handleMessage")

	switch command := msg.Command.(type) {
	case *internal.ExecuteActivity:
		params := command.ActivityParams(wp.env, msg.Payloads)
		activityID := wp.env.ExecuteActivity(params, wp.createCallback(msg.ID))

		wp.canceller.Register(msg.ID, func() error {
			wp.env.RequestCancelActivity(activityID)
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
			Header:                   wp.header,
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

func (wp *process) createCallback(id uint64) bindings.ResultHandler {
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
func (wp *process) createContinuableCallback(id uint64) bindings.ResultHandler {
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
func (wp *process) flushQueue() error {
	const op = errors.Op("flush queue")
	messages, err := wp.codec.Execute(wp.getContext(), wp.mq.Queue...)
	wp.mq.Flush()

	if err != nil {
		return errors.E(op, err)
	}

	wp.pipeline = append(wp.pipeline, messages...)

	return nil
}

// Run single command and return single result.
func (wp *process) runCommand(cmd interface{}, payloads *commonpb.Payloads) (*internal.Message, error) {
	const op = errors.Op("workflow_process_runcommand")
	_, msg := wp.mq.AllocateMessage(cmd, payloads)

	result, err := wp.codec.Execute(wp.getContext(), &msg)
	if err != nil {
		return nil, errors.E(op, err)
	}

	if len(result) != 1 {
		return nil, errors.E(op, errors.Str("unexpected pool response"))
	}

	return result[0], nil
}
