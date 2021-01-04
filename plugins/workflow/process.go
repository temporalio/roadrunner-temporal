package workflow

import (
	"github.com/spiral/errors"
	rrt "github.com/temporalio/roadrunner-temporal"
	commonpb "go.temporal.io/api/common/v1"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/workflow"
	"strconv"
	"sync/atomic"
)

// wraps single workflow process
type workflowProcess struct {
	pool      workflowPool
	env       bindings.WorkflowEnvironment
	header    *commonpb.Header
	mq        *messageQueue
	ids       *idRegistry
	seqID     uint64
	pipeline  []rrt.Message
	callbacks []func() error
	completed bool
	canceller *canceller
}

// Execute workflow, bootstraps process.
func (wf *workflowProcess) Execute(env bindings.WorkflowEnvironment, header *commonpb.Header, input *commonpb.Payloads) {
	wf.env = env
	wf.header = header
	wf.seqID = 0
	wf.canceller = &canceller{}

	// sequenceID shared for all worker workflows
	wf.mq = newMessageQueue(wf.pool.SeqID)
	wf.ids = newIdRegistry()

	env.RegisterCancelHandler(wf.handleCancel)
	env.RegisterSignalHandler(wf.handleSignal)
	env.RegisterQueryHandler(wf.handleQuery)

	cmd := rrt.StartWorkflow{
		Info:  env.WorkflowInfo(),
		Input: []*commonpb.Payload{},
	}

	if input != nil {
		cmd.Input = input.Payloads
	}

	_, err := wf.mq.pushCommand(cmd)

	if err != nil {
		panic(err)
	}
}

// OnWorkflowTaskStarted handles single workflow tick and batch of pipeline from temporal server.
func (wf *workflowProcess) OnWorkflowTaskStarted() {
	var err error
	for _, callback := range wf.callbacks {
		err = callback()
		if err != nil {
			panic(err)
		}
	}
	wf.callbacks = nil

	if err := wf.flushQueue(); err != nil {
		panic(err)
	}

	for len(wf.pipeline) > 0 {
		msg := wf.pipeline[0]
		wf.pipeline = wf.pipeline[1:]

		if msg.IsCommand() {
			err = wf.handleCommand(msg.ID, msg.Command)
		}

		if err != nil {
			panic(err)
		}
	}
}

// StackTrace renders workflow stack trace.
func (wf *workflowProcess) StackTrace() string {
	result, err := wf.runCommand(rrt.GetStackTrace{
		RunID: wf.env.WorkflowInfo().WorkflowExecution.RunID,
	})

	if err != nil {
		return err.Error()
	}

	var stacktrace string
	err = wf.env.GetDataConverter().FromPayload(result.Result[0], &stacktrace)
	if err != nil {
		return err.Error()
	}

	return stacktrace
}

// Close the workflow.
func (wf *workflowProcess) Close() {
	if !wf.completed {
		// offloaded from memory
		_, err := wf.mq.pushCommand(rrt.DestroyWorkflow{
			RunID: wf.env.WorkflowInfo().WorkflowExecution.RunID,
		})

		if err != nil {
			panic(err)
		}
	}

	_, err := wf.discardQueue()
	if err != nil {
		panic(err)
	}
}

// execution context.
func (wf *workflowProcess) getContext() rrt.Context {
	return rrt.Context{
		TaskQueue: wf.env.WorkflowInfo().TaskQueueName,
		TickTime:  wf.env.Now(),
		Replay:    wf.env.IsReplaying(),
	}
}

// schedule cancel command
func (wf *workflowProcess) handleCancel() {
	_, err := wf.mq.pushCommand(rrt.CancelWorkflow{
		RunID: wf.env.WorkflowInfo().WorkflowExecution.RunID,
	})

	if err != nil {
		panic(err)
	}
}

// schedule the signal processing
func (wf *workflowProcess) handleSignal(name string, input *commonpb.Payloads) {
	_, err := wf.mq.pushCommand(rrt.InvokeSignal{
		RunID: wf.env.WorkflowInfo().WorkflowExecution.RunID,
		Name:  name,
		Args:  input.Payloads,
	})

	if err != nil {
		panic(err)
	}
}

// Handle query in blocking mode.
func (wf *workflowProcess) handleQuery(queryType string, queryArgs *commonpb.Payloads) (*commonpb.Payloads, error) {
	result, err := wf.runCommand(rrt.InvokeQuery{
		RunID: wf.env.WorkflowInfo().WorkflowExecution.RunID,
		Name:  queryType,
		Args:  queryArgs.Payloads,
	})

	if err != nil {
		return nil, err
	}

	if result.Error != nil {
		return nil, result.Error
	}

	return &commonpb.Payloads{Payloads: result.Result}, nil
}

// process incoming command
func (wf *workflowProcess) handleCommand(id uint64, cmd interface{}) error {
	const op = errors.Op("handleCommand")
	var err error

	switch cmd := cmd.(type) {
	case *rrt.ExecuteActivity:
		params := cmd.ActivityParams(wf.env)
		activityID := wf.env.ExecuteActivity(params, wf.createCallback(id))

		wf.canceller.register(id, func() error {
			wf.env.RequestCancelActivity(activityID)
			return nil
		})

	case *rrt.ExecuteChildWorkflow:
		params := cmd.WorkflowParams(wf.env)

		// always use deterministic id
		if params.WorkflowID == "" {
			nextID := atomic.AddUint64(&wf.seqID, 1)
			params.WorkflowID = wf.env.WorkflowInfo().WorkflowExecution.RunID + "_" + strconv.Itoa(int(nextID))
		}

		wf.env.ExecuteChildWorkflow(params, wf.createCallback(id), func(r bindings.WorkflowExecution, e error) {
			wf.ids.push(id, r, e)
		})

		wf.canceller.register(id, func() error {
			wf.env.RequestCancelChildWorkflow(params.Namespace, params.WorkflowID)
			return nil
		})

	case *rrt.GetChildWorkflowExecution:
		wf.ids.listen(cmd.ID, func(w bindings.WorkflowExecution, err error) {
			cl := wf.createCallback(id)

			p, err := wf.env.GetDataConverter().ToPayloads(w)
			if err != nil {
				panic(err)
			}

			cl(p, err)
		})

	case *rrt.NewTimer:
		timerID := wf.env.NewTimer(cmd.ToDuration(), wf.createCallback(id))
		wf.canceller.register(id, func() error {
			if timerID != nil {
				wf.env.RequestCancelTimer(*timerID)
			}
			return nil
		})

	case *rrt.GetVersion:
		version := wf.env.GetVersion(
			cmd.ChangeID,
			workflow.Version(cmd.MinSupported),
			workflow.Version(cmd.MaxSupported),
		)

		result, err := wf.env.GetDataConverter().ToPayload(version)
		if err != nil {
			return errors.E(op, err)
		}

		wf.mq.pushResponse(id, []*commonpb.Payload{result})
		err = wf.flushQueue()
		if err != nil {
			panic(err)
		}

	case *rrt.SideEffect:
		wf.env.SideEffect(
			func() (*commonpb.Payloads, error) {
				return &commonpb.Payloads{Payloads: []*commonpb.Payload{cmd.Value}}, nil
			},
			wf.createContinuableCallback(id),
		)

	case *rrt.CompleteWorkflow:
		wf.completed = true

		payload, _ := wf.env.GetDataConverter().ToPayload("completed")
		wf.mq.pushResponse(id, []*commonpb.Payload{payload})

		if cmd.Error == nil {
			wf.env.Complete(&commonpb.Payloads{Payloads: cmd.Result}, nil)
		} else {
			wf.env.Complete(nil, cmd.Error)
		}

	case *rrt.ContinueAsNew:
		wf.completed = true

		payload, _ := wf.env.GetDataConverter().ToPayload("completed")
		wf.mq.pushResponse(id, []*commonpb.Payload{payload})
		wf.env.Complete(nil, errors.E("not implemented"))

	case *rrt.SignalExternalWorkflow:
		wf.env.SignalExternalWorkflow(
			cmd.Namespace,
			cmd.WorkflowID,
			cmd.RunID,
			cmd.Signal,
			&commonpb.Payloads{Payloads: cmd.Args},
			nil,
			cmd.ChildWorkflowOnly,
			wf.createCallback(id),
		)

	case *rrt.CancelExternalWorkflow:
		wf.env.RequestCancelExternalWorkflow(cmd.Namespace, cmd.WorkflowID, cmd.RunID, wf.createCallback(id))

	case *rrt.Cancel:
		err = wf.canceller.cancel(cmd.CommandIDs...)
		if err != nil {
			return errors.E(op, err)
		}

		payload, _ := wf.env.GetDataConverter().ToPayload("completed")
		wf.mq.pushResponse(id, []*commonpb.Payload{payload})

	default:
		panic("undefined command")
	}

	return nil
}

func (wf *workflowProcess) createCallback(id uint64) bindings.ResultHandler {
	callback := func(result *commonpb.Payloads, err error) error {
		wf.canceller.discard(id)

		if err != nil {
			wf.mq.pushError(id, err)
			return nil
		}

		wf.mq.pushPayloadsResponse(id, result)
		return nil
	}

	return func(result *commonpb.Payloads, err error) {
		wf.callbacks = append(wf.callbacks, func() error {
			return callback(result, err)
		})
	}
}

// callback to be called inside the queue processing, adds new messages at the end of the queue
func (wf *workflowProcess) createContinuableCallback(id uint64) bindings.ResultHandler {
	callback := func(result *commonpb.Payloads, err error) {
		wf.canceller.discard(id)

		if err != nil {
			wf.mq.pushError(id, err)
			return
		}

		wf.mq.pushResponse(id, result.Payloads)
		err = wf.flushQueue()
		if err != nil {
			panic(err)
		}
	}

	return func(result *commonpb.Payloads, err error) {
		callback(result, err)
	}
}

// Exchange messages between host and worker processes and add new commands to the queue.
func (wf *workflowProcess) flushQueue() error {
	const op = errors.Op("flush queue")
	messages, err := rrt.Execute(wf.pool, wf.getContext(), wf.mq.queue...)
	wf.mq.flush()

	if err != nil {
		return errors.E(op, err)
	}

	wf.pipeline = append(wf.pipeline, messages...)

	return nil
}

// Exchange messages between host and worker processes without adding new commands to the queue.
func (wf *workflowProcess) discardQueue() ([]rrt.Message, error) {
	const op = errors.Op("discard queue")
	messages, err := rrt.Execute(wf.pool, wf.getContext(), wf.mq.queue...)
	wf.mq.flush()

	if err != nil {
		return nil, errors.E(op, err)
	}

	return messages, nil
}

// Run single command and return single result.
func (wf *workflowProcess) runCommand(cmd interface{}) (rrt.Message, error) {
	const op = errors.Op("run command")
	_, msg, err := wf.mq.allocateMessage(cmd)
	if err != nil {
		return rrt.Message{}, err
	}

	result, err := rrt.Execute(wf.pool, wf.getContext(), msg)
	if err != nil {
		return rrt.Message{}, errors.E(op, err)
	}

	if len(result) != 1 {
		return rrt.Message{}, errors.E("unexpected worker response")
	}

	return result[0], nil
}
