package workflow

import (
	"strconv"
	"sync/atomic"

	jsoniter "github.com/json-iterator/go"
	"github.com/spiral/errors"
	rrt "github.com/temporalio/roadrunner-temporal"
	commonpb "go.temporal.io/api/common/v1"
	bindings "go.temporal.io/sdk/internalbindings"

)

// wraps single workflow process
type workflowProcess struct {
	pool      workflowPool
	env       bindings.WorkflowEnvironment
	mq        *messageQueue
	seqID     uint64
	pipeline  []rrt.Message
	callbacks []func() error
	completed bool
	canceller *canceller
}

// Execute workflow, bootstraps process.
func (wf *workflowProcess) Execute(env bindings.WorkflowEnvironment, header *commonpb.Header, input *commonpb.Payloads) {
	wf.env = env
	wf.seqID = 0
	wf.canceller = &canceller{}

	// sequenceID shared for all worker workflows
	wf.mq = newMessageQueue(wf.pool.SeqID)

	env.RegisterCancelHandler(wf.handleCancel)
	env.RegisterSignalHandler(wf.handleSignal)
	env.RegisterQueryHandler(wf.handleQuery)

	start := StartWorkflow{
		Info: env.WorkflowInfo(),
	}

	err := rrt.FromPayloads(env.GetDataConverter(), input, &start.Input)
	if err != nil {
		panic(err)
	}

	_, err = wf.mq.pushCommand(StartWorkflowCommand, start)
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
			err = wf.handleCommand(msg.ID, msg.Command, msg.Params)
		}

		if err != nil {
			panic(err)
		}
	}
}

// StackTrace renders workflow stack trace.
func (wf *workflowProcess) StackTrace() string {
	result, err := wf.runCommand(
		GetStackTraceCommand,
		&GetStackTrace{RunID: wf.env.WorkflowInfo().WorkflowExecution.RunID},
	)

	if err != nil {
		return err.Error()
	}

	var stacktrace string
	err = jsoniter.Unmarshal(result.Result[0], &stacktrace)

	if err != nil {
		return err.Error()
	}

	return stacktrace
}


		// offloaded from memory
		_, err := wf.mq.pushCommand(
			DestroyWorkflowCommand,
			DestroyWorkflow{RunID: wf.env.WorkflowInfo().WorkflowExecution.RunID},
		)

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
	_, err := wf.mq.pushCommand(
		CancelWorkflowCommand,
		&CancelWorkflow{
			RunID: wf.env.WorkflowInfo().WorkflowExecution.RunID,
		},
	)

	if err != nil {
		panic(err)
	}
}

// schedule the signal processing
func (wf *workflowProcess) handleSignal(name string, input *commonpb.Payloads) {
	cmd := &InvokeQuery{
		RunID: wf.env.WorkflowInfo().WorkflowExecution.RunID,
		Name:  name,
	}

	err := rrt.FromPayloads(wf.env.GetDataConverter(), input, &cmd.Args)
	if err != nil {
		panic(err)
	}

	_, err = wf.mq.pushCommand(InvokeSignalCommand, cmd)
	if err != nil {
		panic(err)
	}
}

// Handle query in blocking mode.
func (wf *workflowProcess) handleQuery(queryType string, queryArgs *commonpb.Payloads) (*commonpb.Payloads, error) {
	cmd := &InvokeQuery{
		RunID: wf.env.WorkflowInfo().WorkflowExecution.RunID,
		Name:  queryType,
	}

	err := rrt.FromPayloads(wf.env.GetDataConverter(), queryArgs, &cmd.Args)
	if err != nil {
		return nil, err
	}

	result, err := wf.runCommand(InvokeQueryCommand, cmd)
	if err != nil {
		return nil, err
	}

	if result.Error != nil {
		return nil, result.Error
	}

	out := &commonpb.Payloads{}
	err = rrt.ToPayloads(wf.env.GetDataConverter(), result.Result, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// process incoming command
func (wf *workflowProcess) handleCommand(id uint64, name string, params jsoniter.RawMessage) error {
	rawCmd, err := parseCommand(wf.env.GetDataConverter(), name, params)
	if err != nil {
		return err
	}

	switch cmd := rawCmd.(type) {
	case ExecuteActivity:
		params := cmd.ActivityParams(wf.env)
		activityID := wf.env.ExecuteActivity(params, wf.createCallback(id))

		wf.canceller.register(id, func() error {
			wf.env.RequestCancelActivity(activityID)
			return nil
		})

	case ExecuteChildWorkflow:
		params := cmd.WorkflowParams(wf.env)

		// always use deterministic id
		if params.WorkflowID == "" {
			nextID := atomic.AddUint64(&wf.seqID, 1)
			params.WorkflowID = wf.env.WorkflowInfo().WorkflowExecution.RunID + "_" + strconv.Itoa(int(nextID))
		}

		wf.env.ExecuteChildWorkflow(params, wf.createCallback(id), func(r bindings.WorkflowExecution, e error) {
			// todo: re
		})

		wf.canceller.register(id, func() error {
			wf.env.RequestCancelChildWorkflow(params.Namespace, params.WorkflowID)
			return nil
		})

	case NewTimer:
		timerID := wf.env.NewTimer(cmd.ToDuration(), wf.createCallback(id))
		wf.canceller.register(id, func() error {
			if timerID != nil {
				wf.env.RequestCancelTimer(*timerID)
			}
			return nil
		})

	case GetVersion:
		version := wf.env.GetVersion(
			cmd.ChangeID,
			workflow.Version(cmd.MinSupported),
			workflow.Version(cmd.MaxSupported),
		)

		result, err := jsoniter.Marshal(version)
		if err != nil {
			return err
		}

		wf.mq.pushResponse(id, []jsoniter.RawMessage{result})
		err = wf.flushQueue()
		if err != nil {
			panic(err)
		}

	case SideEffect:
		wf.env.SideEffect(
			func() (*commonpb.Payloads, error) { return cmd.rawPayload, nil },
			wf.createContinuableCallback(id),
		)

	case CompleteWorkflow:
		wf.completed = true
		wf.mq.pushResponse(id, []jsoniter.RawMessage{[]byte("\"completed\"")})
		if cmd.Error == nil {
			wf.env.Complete(cmd.rawPayload, nil)
		} else {
			wf.env.Complete(nil, cmd.Error)
		}

	case SignalExternalWorkflow:
		wf.env.SignalExternalWorkflow(
			cmd.Namespace,
			cmd.WorkflowID,
			cmd.RunID,
			cmd.Signal,
			cmd.rawPayload,
			nil,
			cmd.ChildWorkflowOnly,
			wf.createCallback(id),
		)

	case CancelExternalWorkflow:
		wf.env.RequestCancelExternalWorkflow(cmd.Namespace, cmd.WorkflowID, cmd.RunID, wf.createCallback(id))

	case Cancel:
		err = wf.canceller.cancel(cmd.CommandIDs...)
		if err != nil {
			return err
		}

		wf.mq.pushResponse(id, []jsoniter.RawMessage{[]byte("\"cancelled\"")})
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

		var data []jsoniter.RawMessage
		err = rrt.FromPayloads(wf.env.GetDataConverter(), result, &data)

		if err != nil {
			panic(err)
		}

		wf.mq.pushResponse(id, data)
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

		var data []jsoniter.RawMessage
		err = rrt.FromPayloads(wf.env.GetDataConverter(), result, &data)
		if err != nil {
			panic(err)
		}

		wf.mq.pushResponse(id, data)
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
	messages, err := rrt.Execute(wf.pool, wf.getContext(), wf.mq.queue...)
	wf.mq.flush()

	if err != nil {
		return err
	}

	wf.pipeline = append(wf.pipeline, messages...)

	return nil
}

// Exchange messages between host and worker processes without adding new commands to the queue.
func (wf *workflowProcess) discardQueue() ([]rrt.Message, error) {
	messages, err := rrt.Execute(wf.pool, wf.getContext(), wf.mq.queue...)
	wf.mq.flush()

	if err != nil {
		return nil, err
	}

	return messages, nil
}

// Run single command and return single result.
func (wf *workflowProcess) runCommand(command string, params interface{}) (rrt.Message, error) {
	_, msg, err := wf.mq.makeCommand(command, params)

	result, err := rrt.Execute(wf.pool, wf.getContext(), msg)
	if err != nil {
		return rrt.Message{}, err
	}

	if len(result) != 1 {
		return rrt.Message{}, errors.E("unexpected worker response")
	}

	return result[0], nil
}
