package workflow

import (
	"encoding/json"
	"github.com/spiral/roadrunner/v2"
	rrt "github.com/temporalio/roadrunner-temporal"
	commonpb "go.temporal.io/api/common/v1"
	bindings "go.temporal.io/sdk/internalbindings"
	"log"
	"sync/atomic"
	"time"
)

type workflowProcess struct {
	pool      *workflowPool
	taskQueue string
	worker    roadrunner.SyncWorker
	env       bindings.WorkflowEnvironment
	// todo: pre-fetch workflows
	// todo: improve data conversions
	queue     []rrt.Message
	callbacks []func()
	complete  bool
}

func (wp *workflowProcess) Execute(env bindings.WorkflowEnvironment, header *commonpb.Header, input *commonpb.Payloads) {
	wp.callbacks = append(wp.callbacks, func() {
		info := env.WorkflowInfo()

		wp.env = env
		wp.taskQueue = info.TaskQueueName

		// todo: update mapping
		wp.pushCommand("StartWorkflow", struct {
			Name      string             `json:"name"`
			Wid       string             `json:"wid"`
			Rid       string             `json:"rid"`
			TaskQueue string             `json:"taskQueue"`
			Payload   *commonpb.Payloads `json:"payload"`
		}{
			Name:      info.WorkflowType.Name,
			Wid:       info.WorkflowExecution.ID,
			Rid:       info.WorkflowExecution.RunID,
			TaskQueue: info.TaskQueueName,
			Payload:   input, // todo: implement
		})
	})
}

func (wp *workflowProcess) OnWorkflowTaskStarted() {
	for _, callback := range wp.callbacks {
		callback()
	}
	wp.callbacks = nil

	if wp.queue != nil {
		result, err := wp.execute(wp.queue...)
		if err != nil {
			// todo: what to do?
			log.Println("error: ", err)
		}
		wp.queue = nil

		for _, frame := range result {
			if frame.Command == "" {
				//				log.Printf("got response for %v: %s", frame.ID, color.MagentaString(string(frame.Result)))
				continue
			}

			//			log.Printf("got command for %s(%v): %s", color.BlueString(frame.Command), frame.ID, color.GreenString(string(frame.Params)))
			wp.handle(frame.Command, frame.ID, frame.Params)
		}
	}
}

func (wp *workflowProcess) handle(cmd string, id uint64, params json.RawMessage) error {
	switch cmd {
	case "ExecuteActivity":
		data := ExecuteActivity{}
		err := json.Unmarshal(params, &data)
		if err != nil {
			return err
		}

		payloads, err := wp.env.GetDataConverter().ToPayloads(data.Args...)
		if err != nil {
			return err
		}

		// todo: get options from activity, improve mapping
		options := bindings.ExecuteActivityParams{
			ExecuteActivityOptions: bindings.ExecuteActivityOptions{
				TaskQueueName:          wp.taskQueue,
				ScheduleToCloseTimeout: time.Second * 60,
				ScheduleToStartTimeout: time.Second * 60,
				StartToCloseTimeout:    time.Second * 60,
				HeartbeatTimeout:       time.Second * 10,
			},
			ActivityType: bindings.ActivityType{Name: data.Name},
			Input:        payloads,
		}

		wp.env.ExecuteActivity(options, wp.newResultHandler(id))

	case "NewTimer":
		data := NewTimer{}
		err := json.Unmarshal(params, &data)
		if err != nil {
			return err
		}

		wp.env.NewTimer(data.ToDuration(), wp.newResultHandler(id))

	case "CompleteWorkflow":
		data := CompleteWorkflow{}
		err := json.Unmarshal(params, &data)
		if err != nil {
			return err
		}

		payloads, err := wp.env.GetDataConverter().ToPayloads(data.Result...)
		if err != nil {
			return err
		}

		//log.Println("complete")
		wp.env.Complete(payloads, nil)

		// confirm it
		wp.pushResult(id, &commonpb.Payloads{})
		wp.complete = true
	}

	return nil
}

func (wp *workflowProcess) StackTrace() string {
	// TODO: IDEAL - debug_stacktrace()
	return "this is ST"
}

func (wp *workflowProcess) Close() {
	atomic.AddUint64(&wp.pool.numW, ^uint64(0))
	if wp.queue != nil {
		wp.execute(wp.queue...)
	}

	if !wp.complete {
		log.Println("OFFLOAD NOT COMPELETE")
	}

	// TODO: CACHE OFFLOAD
	// todo: send command
}

func (wp *workflowProcess) newResultHandler(id uint64) bindings.ResultHandler {
	return wp.addCallback(func(result *commonpb.Payloads, err error) {
		if err != nil {
			wp.pushError(id, err)
		} else {
			wp.pushResult(id, result)
		}
	})
}

func (wp *workflowProcess) addCallback(callback bindings.ResultHandler) bindings.ResultHandler {
	return func(result *commonpb.Payloads, err error) {
		wp.callbacks = append(wp.callbacks, func() {
			callback(result, err)
		})
	}
}

func (wp *workflowProcess) pushCommand(name string, params interface{}) (id uint64, err error) {
	cmd := rrt.Message{
		ID:      atomic.AddUint64(&wp.pool.seqID, 1),
		Command: name,
	}

	cmd.Params, err = json.Marshal(params)
	if err != nil {
		return 0, err
	}

	wp.queue = append(wp.queue, cmd)

	return id, nil
}

func (wp *workflowProcess) pushResult(id uint64, result *commonpb.Payloads) {
	cmd := rrt.Message{
		ID: id,
	}

	payload := rrt.RRPayload{}
	wp.env.GetDataConverter().FromPayloads(result, &payload)

	// todo: REPACK
	//	cmd.Result, _ = json.Marshal(payload.Data)
	wp.queue = append(wp.queue, cmd)
}

func (wp *workflowProcess) pushError(id uint64, err error) {
	//	cmd := rrt.Message{
	//		ID:    id,
	//		Error: err.Error(),
	//}
	//log.Println("error!", err)
	//wp.queue = append(wp.queue, cmd)

}

type Wrapper struct {
	Rid      string        `json:"rid"`
	Commands []rrt.Message `json:"commands"`
}

// Exchange commands with worker.
func (wp *workflowProcess) execute(cmd ...rrt.Message) (result []rrt.Message, err error) {
	atomic.AddUint64(&wp.pool.numS, 1)
	defer atomic.AddUint64(&wp.pool.numS, ^uint64(0))

	ctx := rrt.Context{
		TaskQueue: wp.taskQueue,
		Replay:    wp.env.IsReplaying(),
		TickTime:  wp.env.Now(),
	}

	wr := Wrapper{
		Rid:      wp.env.WorkflowInfo().WorkflowExecution.RunID,
		Commands: cmd,
	}

	p := roadrunner.Payload{}
	if p.Context, err = json.Marshal(ctx); err != nil {
		return nil, err
	}

	p.Body, err = json.Marshal(wr)
	if err != nil {
		return nil, err
	}

	//log.Printf("send: %s", color.YellowString(string(p.Body)))

	rsp, err := wp.pool.Exec(p)
	if err != nil {
		return nil, err
	}

	wrr := Wrapper{}

	err = json.Unmarshal(rsp.Body, &wrr)
	if err != nil {
		return nil, err
	}

	return wrr.Commands, nil
}
