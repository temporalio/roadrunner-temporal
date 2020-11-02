package workflow

import (
	"encoding/json"
	"fmt"
	rrt "github.com/temporalio/roadrunner-temporal"
	commonpb "go.temporal.io/api/common/v1"
	bindings "go.temporal.io/sdk/internalbindings"
)

// wraps single workflow process
type workflowProcess struct {
	pool      *workflowPool
	env       bindings.WorkflowEnvironment
	mq        *messageQueue
	callbacks []func() error
	completed bool
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

// OnWorkflowTaskStarted handles single workflow tick and batch of messages from temporal server.
func (wp *workflowProcess) OnWorkflowTaskStarted() {
	var err error
	for _, callback := range wp.callbacks {
		err = callback()
		if err != nil {
			panic(err)
		}
	}
	wp.callbacks = nil

	messages, err := rrt.Execute(wp.pool, wp.getContext(), wp.mq.queue...)
	wp.mq.flush()
	if err != nil {
		panic(err)
	}

	for _, msg := range messages {
		if msg.Command == "" {
			// handle responses, unclear if we need it here
			continue
		}

		if err := wp.handleCommand(msg.ID, msg.Command, msg.Params); err != nil {
			panic(err)
		}
	}
}

func (wp *workflowProcess) StackTrace() string {
	// TODO: IDEAL - debug_stacktrace()

	return "todo: needs to be implemented"
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

	_, err := rrt.Execute(wp.pool, wp.getContext(), wp.mq.queue...)
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

// todo: MUST BE SCHEDULED
func (wp *workflowProcess) handleQuery(queryType string, queryArgs *commonpb.Payloads) (*commonpb.Payloads, error) {
	cmd := &InvokeQuery{
		RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID,
		Name:  queryType,
	}

	if err := rrt.FromPayloads(wp.env.GetDataConverter(), queryArgs, &cmd.Args); err != nil {
		return nil, err
	}

	// we can trigger signal immediately on arrival, todo: double check that
	_, msg, err := wp.mq.makeCommand(InvokeQueryCommand, cmd)

	result, err := rrt.Execute(wp.pool, wp.getContext(), msg)
	if err != nil {
		return nil, err
	}

	if len(result) != 1 {
		return nil, fmt.Errorf("unexpected worker response")
	}

	out := &commonpb.Payloads{}
	if err := rrt.ToPayloads(wp.env.GetDataConverter(), result[0].Result, out); err != nil {
		return nil, err
	}

	return out, nil
}

// todo: schedule?
func (wp *workflowProcess) handleSignal(name string, input *commonpb.Payloads) {
	cmd := &InvokeQuery{
		RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID,
		Name:  name,
	}

	if err := rrt.FromPayloads(wp.env.GetDataConverter(), input, &cmd.Args); err != nil {
		// todo: what to do about this panic?
		panic(err)
	}

	// we can trigger signal immediately on arrival, todo: double check that
	_, msg, err := wp.mq.makeCommand(InvokeSignalCommand, cmd)

	_, err = rrt.Execute(wp.pool, wp.getContext(), msg)
	if err != nil {
		panic(err)
	}
}

func (wp *workflowProcess) handleCommand(id uint64, name string, params json.RawMessage) error {
	rawCmd, err := parseCommand(wp.env.GetDataConverter(), name, params)
	if err != nil {
		return err
	}

	switch cmd := rawCmd.(type) {
	case ExecuteActivity:
		wp.env.ExecuteActivity(cmd.ActivityParams(wp.env), wp.createCallback(id))

	case NewTimer:
		wp.env.NewTimer(cmd.ToDuration(), wp.createCallback(id))

	case CompleteWorkflow:
		wp.completed = true
		wp.mq.pushResponse(id, []json.RawMessage{[]byte("true")})
		wp.env.Complete(cmd.ResultPayload, nil)
	}

	return nil
}

func (wp *workflowProcess) createCallback(id uint64) bindings.ResultHandler {
	callback := func(result *commonpb.Payloads, err error) error {
		if err != nil {
			wp.mq.pushError(id, err)
			return nil
		}

		var data []json.RawMessage
		if err = rrt.FromPayloads(wp.env.GetDataConverter(), result, &data); err != nil {
			return err
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
