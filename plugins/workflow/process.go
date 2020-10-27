package workflow

import (
	"encoding/json"
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

func (wp *workflowProcess) Close() {
	// TODO: detect if workflow has to be offloaded before the completion, send terminate process command
}

func (wp *workflowProcess) getContext() rrt.Context {
	return rrt.Context{
		TaskQueue: wp.env.WorkflowInfo().TaskQueueName,
		TickTime:  wp.env.Now(),
		Replay:    wp.env.IsReplaying(),
	}
}

func (wp *workflowProcess) createCallback(id uint64) bindings.ResultHandler {
	callback := func(result *commonpb.Payloads, err error) error {
		if err != nil {
			wp.mq.pushError(id, err)
			return nil
		}

		var data []json.RawMessage
		if err = rrt.FromPayload(wp.env.GetDataConverter(), result, &data); err != nil {
			return err
		}

		wp.mq.pushResult(id, data)
		return nil
	}

	return func(result *commonpb.Payloads, err error) {
		wp.callbacks = append(wp.callbacks, func() error {
			return callback(result, err)
		})
	}
}

func (wp *workflowProcess) handleCommand(id uint64, name string, params json.RawMessage) error {
	cmd, err := parseCommand(wp.env.GetDataConverter(), name, params)
	if err != nil {
		return err
	}

	return nil
	//switch name {
	//case "ExecuteActivity":
	//	data := ExecuteActivity{}
	//	err := json.Unmarshal(params, &data)
	//	if err != nil {
	//		return err
	//	}
	//
	//	payloads, err := wp.env.GetDataConverter().ToPayloads(data.Input...)
	//	if err != nil {
	//		return err
	//	}
	//
	//	// todo: get options from activity, improve mapping
	//	options := bindings.ExecuteActivityParams{
	//		ExecuteActivityOptions: bindings.ExecuteActivityOptions{
	//			TaskQueueName:          wp.env.WorkflowInfo().TaskQueueName,
	//			ScheduleToCloseTimeout: time.Second * 60,
	//			ScheduleToStartTimeout: time.Second * 60,
	//			StartToCloseTimeout:    time.Second * 60,
	//			HeartbeatTimeout:       time.Second * 10,
	//		},
	//		ActivityType: bindings.ActivityType{Name: data.Name},
	//		Input:        payloads,
	//	}
	//
	//	wp.env.ExecuteActivity(options, wp.createCallback(id))
	//
	//case "NewTimer":
	//	data := NewTimer{}
	//	err := json.Unmarshal(params, &data)
	//	if err != nil {
	//		return err
	//	}
	//
	//	wp.env.NewTimer(data.ToDuration(), wp.createCallback(id))
	//
	//case "CompleteWorkflow":
	//	data := CompleteWorkflow{}
	//	err := json.Unmarshal(params, &data)
	//	if err != nil {
	//		return err
	//	}
	//
	//	payloads, err := wp.env.GetDataConverter().ToPayloads(data.Result...)
	//	if err != nil {
	//		return err
	//	}
	//
	//	//log.Println("complete")
	//	wp.env.Complete(payloads, nil)
	//
	//	// confirm it
	//	wp.pushResult(id, &commonpb.Payloads{})
	//}
	//
	//return nil
}
