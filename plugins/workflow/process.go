package workflow

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	rrt "github.com/temporalio/roadrunner-temporal"
	commonpb "go.temporal.io/api/common/v1"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/workflow"
)

// wraps single workflow process
type workflowProcess struct {
	pool      *workflowPool
	env       bindings.WorkflowEnvironment
	mq        *messageQueue
	pipeline  []rrt.Message
	callbacks []func() error
	completed bool
	// queue
}

// Execute workflow, bootstraps process.
func (wp *workflowProcess) Execute(env bindings.WorkflowEnvironment, header *commonpb.Header, input *commonpb.Payloads) {
	wp.callbacks = append(wp.callbacks, func() error {
		wp.env = env
		wp.mq = newMessageQueue(&wp.pool.seqID)

		start := StartWorkflow{}
		if err := start.FromEnvironment(env, input); err != nil {
			return err
		}

		env.RegisterSignalHandler(wp.handleSignal)
		env.RegisterQueryHandler(wp.handleQuery)

		_, err := wp.mq.pushCommand(StartWorkflowCommand, start)
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
		} else {
			err = wp.handleResponse(msg.ID, msg.Result, msg.Error)
		}

		if err != nil {
			panic(err)
		}
	}
}

// StackTrace renders workflow stack trace.
func (wp *workflowProcess) StackTrace() string {
	result, err := wp.runCommand(
		StackTraceCommand,
		&GetBacktrace{RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID},
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
	if !wp.completed {
		//offloaded from memory
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

func (wp *workflowProcess) getContext() rrt.Context {
	return rrt.Context{
		TaskQueue: wp.env.WorkflowInfo().TaskQueueName,
		TickTime:  wp.env.Now(),
		Replay:    wp.env.IsReplaying(),
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

// schedule the signal processing
func (wp *workflowProcess) handleSignal(name string, input *commonpb.Payloads) {
	cmd := &InvokeQuery{
		RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID,
		Name:  name,
	}

	err := rrt.FromPayloads(wp.env.GetDataConverter(), input, &cmd.Args)
	if err != nil {
		// todo: what to do about this panic?
		panic(err)
	}

	_, err = wp.mq.pushCommand(InvokeSignalCommand, cmd)
	if err != nil {
		panic(err)
	}
}

// process incoming command
func (wp *workflowProcess) handleCommand(id uint64, name string, params jsoniter.RawMessage) error {
	rawCmd, err := parseCommand(wp.env.GetDataConverter(), name, params)
	if err != nil {
		return err
	}

	switch cmd := rawCmd.(type) {
	case ExecuteActivity:
		wp.env.ExecuteActivity(cmd.ActivityParams(wp.env), wp.createCallback(id))

	case NewTimer:
		wp.env.NewTimer(cmd.ToDuration(), wp.createCallback(id))

	case GetVersion:
		version := wp.env.GetVersion(cmd.ChangeID, workflow.Version(cmd.MinSupported), workflow.Version(cmd.MaxSupported))

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
			func() (*commonpb.Payloads, error) { return cmd.Payloads, nil },
			wp.createBlockingCallback(id),
		)

	case CompleteWorkflow:
		wp.completed = true
		wp.mq.pushResponse(id, []jsoniter.RawMessage{[]byte("true")})
		wp.env.Complete(cmd.ResultPayload, nil)
	}

	return nil
}

// process incoming command
func (wp *workflowProcess) handleResponse(id uint64, result []jsoniter.RawMessage, error interface{}) error {
	return nil
}

func (wp *workflowProcess) createCallback(id uint64) bindings.ResultHandler {
	callback := func(result *commonpb.Payloads, err error) error {
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
func (wp *workflowProcess) createBlockingCallback(id uint64) bindings.ResultHandler {
	callback := func(result *commonpb.Payloads, err error) {
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

// Exchange messages between host and worker processes.
func (wp *workflowProcess) flushQueue() error {
	messages, err := rrt.Execute(wp.pool, wp.getContext(), wp.mq.queue...)
	wp.mq.flush()
	if err != nil {
		return err
	}

	wp.pipeline = append(wp.pipeline, messages...)

	return nil
}

// Exchange messages between host and worker processes without adding them to queue.
func (wp *workflowProcess) discardQueue() ([]rrt.Message, error) {
	messages, err := rrt.Execute(wp.pool, wp.getContext(), wp.mq.queue...)
	wp.mq.flush()
	if err != nil {
		return nil, err
	}

	return messages, nil
}

// Exchange messages between host and worker processes without adding them to queue.
func (wp *workflowProcess) runCommand(command string, params interface{}) (rrt.Message, error) {
	_, msg, err := wp.mq.makeCommand(command, params)

	result, err := rrt.Execute(wp.pool, wp.getContext(), msg)
	if err != nil {
		return rrt.Message{}, err
	}

	if len(result) != 1 {
		return rrt.Message{}, fmt.Errorf("unexpected worker response")
	}

	return result[0], nil
}
