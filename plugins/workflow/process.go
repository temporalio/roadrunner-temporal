package workflow

import (
	jsoniter "github.com/json-iterator/go"
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
	mq        *messageQueue
	seqID     uint64
	pipeline  []rrt.Message
	callbacks []func() error
	completed bool
	canceller *canceller
}

// Execute workflow, bootstraps process.
func (wp *workflowProcess) Execute(env bindings.WorkflowEnvironment, header *commonpb.Header, input *commonpb.Payloads) {
	wp.env = env
	wp.seqID = 0
	wp.canceller = &canceller{}

	// sequenceID shared for all worker workflows
	wp.mq = newMessageQueue(wp.pool.SeqID)

	wp.callbacks = append(wp.callbacks, func() error {
		start := StartWorkflow{
			Info: env.WorkflowInfo(),
		}

		err := rrt.FromPayloads(env.GetDataConverter(), input, &start.Input)
		if err != nil {
			return err
		}

		env.RegisterCancelHandler(wp.handleCancel)
		env.RegisterSignalHandler(wp.handleSignal)
		env.RegisterQueryHandler(wp.handleQuery)

		_, err = wp.mq.pushCommand(StartWorkflowCommand, start)
		return err
	})
}

// OnWorkflowTaskStarted handles single workflow tick and batch of pipeline from temporal server.
func (wp *workflowProcess) OnWorkflowTaskStarted() {
	var err error
	for _, callback := range wp.callbacks {
		err = callback()
		if err != nil {
			panic(err)
		}
	}
	wp.callbacks = nil

	if err := wp.flushQueue(); err != nil {
		panic(err)
	}

	for len(wp.pipeline) > 0 {
		msg := wp.pipeline[0]
		wp.pipeline = wp.pipeline[1:]

		if msg.IsCommand() {
			err = wp.handleCommand(msg.ID, msg.Command, msg.Params)
		}

		if err != nil {
			panic(err)
		}
	}
}

// StackTrace renders workflow stack trace.
func (wp *workflowProcess) StackTrace() string {
	result, err := wp.runCommand(
		GetStackTraceCommand,
		&GetStackTrace{RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID},
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

// Close the workflow.
func (wp *workflowProcess) Close() {
	if wp.env == nil {
		return
	}

	if !wp.completed {
		// offloaded from memory
		_, err := wp.mq.pushCommand(
			DestroyWorkflowCommand,
			DestroyWorkflow{RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID},
		)

		if err != nil {
			panic(err)
		}
	}

	_, err := wp.discardQueue()
	if err != nil {
		panic(err)
	}
}

// execution context.
func (wp *workflowProcess) getContext() rrt.Context {
	return rrt.Context{
		TaskQueue: wp.env.WorkflowInfo().TaskQueueName,
		TickTime:  wp.env.Now(),
		Replay:    wp.env.IsReplaying(),
	}
}

// schedule cancel command
func (wp *workflowProcess) handleCancel() {
	_, err := wp.mq.pushCommand(
		CancelWorkflowCommand,
		&CancelWorkflow{
			RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID,
		},
	)

	if err != nil {
		panic(err)
	}
}

// schedule the signal processing
func (wp *workflowProcess) handleSignal(name string, input *commonpb.Payloads) {
	cmd := &InvokeQuery{
		RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID,
		Name:  name,
	}

	err := rrt.FromPayloads(wp.env.GetDataConverter(), input, &cmd.Args)
	if err != nil {
		panic(err)
	}

	_, err = wp.mq.pushCommand(InvokeSignalCommand, cmd)
	if err != nil {
		panic(err)
	}
}

// Handle query in blocking mode.
func (wp *workflowProcess) handleQuery(queryType string, queryArgs *commonpb.Payloads) (*commonpb.Payloads, error) {
	cmd := &InvokeQuery{
		RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID,
		Name:  queryType,
	}

	err := rrt.FromPayloads(wp.env.GetDataConverter(), queryArgs, &cmd.Args)
	if err != nil {
		return nil, err
	}

	result, err := wp.runCommand(InvokeQueryCommand, cmd)
	if err != nil {
		return nil, err
	}

	if result.Error != nil {
		return nil, result.Error
	}

	out := &commonpb.Payloads{}
	err = rrt.ToPayloads(wp.env.GetDataConverter(), result.Result, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// process incoming command
func (wp *workflowProcess) handleCommand(id uint64, name string, params jsoniter.RawMessage) error {
	rawCmd, err := parseCommand(wp.env.GetDataConverter(), name, params)
	if err != nil {
		return err
	}

	switch cmd := rawCmd.(type) {
	case ExecuteActivity:
		params := cmd.ActivityParams(wp.env)
		activityID := wp.env.ExecuteActivity(params, wp.createCallback(id))

		wp.canceller.register(id, func() error {
			wp.env.RequestCancelActivity(activityID)
			return nil
		})

	case ExecuteChildWorkflow:
		params := cmd.WorkflowParams(wp.env)

		// always use deterministic id
		if params.WorkflowID == "" {
			nextID := atomic.AddUint64(&wp.seqID, 1)
			params.WorkflowID = wp.env.WorkflowInfo().WorkflowExecution.RunID + "_" + strconv.Itoa(int(nextID))
		}

		wp.env.ExecuteChildWorkflow(params, wp.createCallback(id), func(r bindings.WorkflowExecution, e error) {
			// todo: re
		})

		wp.canceller.register(id, func() error {
			wp.env.RequestCancelChildWorkflow(params.Namespace, params.WorkflowID)
			return nil
		})

	case NewTimer:
		timerID := wp.env.NewTimer(cmd.ToDuration(), wp.createCallback(id))
		wp.canceller.register(id, func() error {
			if timerID != nil {
				wp.env.RequestCancelTimer(*timerID)
			}
			return nil
		})

	case GetVersion:
		version := wp.env.GetVersion(
			cmd.ChangeID,
			workflow.Version(cmd.MinSupported),
			workflow.Version(cmd.MaxSupported),
		)

		result, err := jsoniter.Marshal(version)
		if err != nil {
			return err
		}

		wp.mq.pushResponse(id, []jsoniter.RawMessage{result})
		err = wp.flushQueue()
		if err != nil {
			panic(err)
		}

	case SideEffect:
		wp.env.SideEffect(
			func() (*commonpb.Payloads, error) { return cmd.rawPayload, nil },
			wp.createContinuableCallback(id),
		)

	case CompleteWorkflow:
		wp.completed = true
		wp.mq.pushResponse(id, []jsoniter.RawMessage{[]byte("\"completed\"")})
		if cmd.Error == nil {
			wp.env.Complete(cmd.rawPayload, nil)
		} else {
			wp.env.Complete(nil, cmd.Error)
		}

	case SignalExternalWorkflow:
		wp.env.SignalExternalWorkflow(
			cmd.Namespace,
			cmd.WorkflowID,
			cmd.RunID,
			cmd.Signal,
			cmd.rawPayload,
			nil,
			cmd.ChildWorkflowOnly,
			wp.createCallback(id),
		)

	case CancelExternalWorkflow:
		wp.env.RequestCancelExternalWorkflow(cmd.Namespace, cmd.WorkflowID, cmd.RunID, wp.createCallback(id))

	case Cancel:
		err = wp.canceller.cancel(cmd.CommandIDs...)
		if err != nil {
			return err
		}
	}

	return nil
}

func (wp *workflowProcess) createCallback(id uint64) bindings.ResultHandler {
	callback := func(result *commonpb.Payloads, err error) error {
		wp.canceller.discard(id)

		if err != nil {
			wp.mq.pushError(id, err)
			return nil
		}

		var data []jsoniter.RawMessage
		err = rrt.FromPayloads(wp.env.GetDataConverter(), result, &data)

		if err != nil {
			panic(err)
		}

		wp.mq.pushResponse(id, data)
		return nil
	}

	return func(result *commonpb.Payloads, err error) {
		wp.callbacks = append(wp.callbacks, func() error {
			return callback(result, err)
		})
	}
}

// callback to be called inside the queue processing, adds new messages at the end of the queue
func (wp *workflowProcess) createContinuableCallback(id uint64) bindings.ResultHandler {
	callback := func(result *commonpb.Payloads, err error) {
		wp.canceller.discard(id)

		if err != nil {
			wp.mq.pushError(id, err)
			return
		}

		var data []jsoniter.RawMessage
		err = rrt.FromPayloads(wp.env.GetDataConverter(), result, &data)
		if err != nil {
			panic(err)
		}

		wp.mq.pushResponse(id, data)
		err = wp.flushQueue()
		if err != nil {
			panic(err)
		}
	}

	return func(result *commonpb.Payloads, err error) {
		callback(result, err)
	}
}

// Exchange messages between host and worker processes and add new commands to the queue.
func (wp *workflowProcess) flushQueue() error {
	messages, err := rrt.Execute(wp.pool, wp.getContext(), wp.mq.queue...)
	wp.mq.flush()

	if err != nil {
		return err
	}

	wp.pipeline = append(wp.pipeline, messages...)

	return nil
}

// Exchange messages between host and worker processes without adding new commands to the queue.
func (wp *workflowProcess) discardQueue() ([]rrt.Message, error) {
	messages, err := rrt.Execute(wp.pool, wp.getContext(), wp.mq.queue...)
	wp.mq.flush()

	if err != nil {
		return nil, err
	}

	return messages, nil
}

// Run single command and return single result.
func (wp *workflowProcess) runCommand(command string, params interface{}) (rrt.Message, error) {
	_, msg, err := wp.mq.makeCommand(command, params)

	result, err := rrt.Execute(wp.pool, wp.getContext(), msg)
	if err != nil {
		return rrt.Message{}, err
	}

	if len(result) != 1 {
		return rrt.Message{}, errors.E("unexpected worker response")
	}

	return result[0], nil
}
