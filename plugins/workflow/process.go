package workflow

import (
	"encoding/json"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	commonpb "go.temporal.io/api/common/v1"
	bindings "go.temporal.io/sdk/internalbindings"
)

// wraps single workflow process
type workflowProcess struct {
	pool      *workflowPool
	env       bindings.WorkflowEnvironment
	mq        *messageQueue
	callbacks []func()
}

// Execute workflow, bootstraps process.
func (wp *workflowProcess) Execute(env bindings.WorkflowEnvironment, header *commonpb.Header, input *commonpb.Payloads) {
	wp.callbacks = append(wp.callbacks, func() {
		wp.env = env
		wp.mq = newMessageQueue(&wp.pool.seqID)

		start := StartWorkflow{}
		if err := start.FromEnvironment(env, input); err != nil {
			panic(err)
		}

		if _, err := wp.mq.pushCommand("StartWorkflow", start); err != nil {
			panic(err)
		}
	})
}

// OnWorkflowTaskStarted handles single workflow tick and batch of messages from temporal server.
func (wp *workflowProcess) OnWorkflowTaskStarted() {
	for _, callback := range wp.callbacks {
		callback()
	}
	wp.callbacks = nil

	//if wp.mq != nil {
	//	result, err := wp.execute(wp.mq...)
	//	if err != nil {
	//		// todo: what to do?
	//		log.Println("error: ", err)
	//	}
	//	wp.mq = nil
	//
	//	for _, frame := range result {
	//		if frame.Command == "" {
	//			//				log.Printf("got response for %v: %s", frame.ID, color.MagentaString(string(frame.Result)))
	//			continue
	//		}
	//
	//		//			log.Printf("got command for %s(%v): %s", color.BlueString(frame.Command), frame.ID, color.GreenString(string(frame.Params)))
	//		wp.handle(frame.Command, frame.ID, frame.Params)
	//	}
	//}
}

func (wp *workflowProcess) StackTrace() string {
	// TODO: IDEAL - debug_stacktrace()
	return "this is ST"
}

func (wp *workflowProcess) Close() {
	// TODO: detect if workflow has to be offloaded before the completion, send terminate process command
}

func (wp *workflowProcess) createCallback(id uint64) bindings.ResultHandler {
	callback := func(result *commonpb.Payloads, err error) {
		if err != nil {
			wp.mq.pushError(id, err)
		} else {
			var data []json.RawMessage
			if err = temporal.ParsePayload(wp.env.GetDataConverter(), result, &data); err != nil {
				panic(err)
			}
			if err = wp.mq.pushResult(id, data); err != nil {
				panic(err)
			}
		}
	}

	return func(result *commonpb.Payloads, err error) {
		wp.callbacks = append(wp.callbacks, func() {
			callback(result, err)
		})
	}
}

func (wp *workflowProcess) handle(cmd string, id uint64, params json.RawMessage) error {
	return nil
	//switch cmd {
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

// Exchange commands with worker.
//func (wp *workflowProcess) execute(cmd ...rrt.Message)// (result []rrt.Message, err error) {
//ctx := rrt.Context{
//	TaskQueue: wp.taskQueue,
//	Replay:    wp.env.IsReplaying(),
//	TickTime:  wp.env.Now(),
//}
//
//wr := Wrapper{
//	Rid:      wp.env.WorkflowInfo().WorkflowExecution.RunID,
//	Commands: cmd,
//}
//
//p := roadrunner.Payload{}
//if p.Context, err = json.Marshal(ctx); err != nil {
//	return nil, err
//}
//
//p.Body, err = json.Marshal(wr)
//if err != nil {
//	return nil, err
//}
//
////log.Printf("send: %s", color.YellowString(string(p.Body)))
//
//rsp, err := wp.pool.Exec(p)
//if err != nil {
//	return nil, err
//}
//
//wrr := Wrapper{}
//
//err = json.Unmarshal(rsp.Body, &wrr)
//if err != nil {
//	return nil, err
//}
//
//return wrr.Commands, nil
//}
