package workflow

import (
	"encoding/json"
	"github.com/fatih/color"
	"github.com/spiral/roadrunner/v2"
	rrt "github.com/temporalio/roadrunner-temporal"
	commonpb "go.temporal.io/api/common/v1"
	bindings "go.temporal.io/sdk/internalbindings"
	"log"
	"sync/atomic"
	"time"
)

type workflowProxy struct {
	seqID     *uint64
	taskQueue string
	worker    roadrunner.SyncWorker
	env       bindings.WorkflowEnvironment
	queue     []rrt.Frame
	callbacks []func()
}

func (wp *workflowProxy) Execute(env bindings.WorkflowEnvironment, header *commonpb.Header, input *commonpb.Payloads) {
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

func (wp *workflowProxy) OnWorkflowTaskStarted() {
	log.Print("task started")
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
				log.Printf("got response for %v: %s", frame.ID, color.MagentaString(string(frame.Result)))
				continue
			}

			log.Printf("got command for %s(%v): %s", color.BlueString(frame.Command), frame.ID, color.GreenString(string(frame.Params)))
			wp.handle(frame.Command, frame.ID, frame.Params)
		}
	}

	//dc := NewRRDataConverter()
	//for {
	//	select {
	//	case sig := <-wp.workflowSignals:
	//		log.Printf(
	//			"[debug] got worflow specific signal %s: %s",
	//			color.RedString(sig.Method),
	//			color.MagentaString(string(sig.Data)),
	//		)
	//
	//		switch sig.Method {
	//		case ExecuteActivity:
	//			ex := new(ExecuteActivityStr)
	//			err := json.Unmarshal(sig.Data, ex)
	//
	//			if err != nil {
	//				panic(err)
	//			}
	//
	//			input, err := dc.ToPayloads(ex)
	//			if err != nil {
	//				panic(err)
	//			}
	//
	//			parameters := bindings.ExecuteActivityParams{
	//				ExecuteActivityOptions: bindings.ExecuteActivityOptions{
	//					TaskQueueName:       wp.env.WorkflowInfo().TaskQueueName,
	//					StartToCloseTimeout: 10 * time.Second,
	//				},
	//				// activity type: NAME here is the function name to invoke
	//				ActivityType: bindings.ActivityType{Name: ex.Params.Name},
	//				Input:        input, // todo: use one from .ex, convert from JSON to Payloads
	//			}
	//
	//			// todo: where is our message ID?
	//			wp.env.ExecuteActivity(parameters, wp.addCallback(func(result *commonpb.Payloads, err error) {
	//				// todo: use callbacks?
	//				if err != nil {
	//					// ERROR RESPONSE
	//					errResp := ErrorResponse{
	//						Error: struct {
	//							Code    string `json:"code"`
	//							Message string `json:"message"`
	//						}{
	//							Code:    "666",
	//							Message: err.Error(),
	//						},
	//						ID: sig.ID,
	//					}
	//					wp.queue = append(wp.queue, errResp)
	//					log.Printf("ACTIVITIY DONE WITH ERRORS")
	//					return
	//				}
	//
	//				// RESPONSE
	//				valPtr := &RrResult{}
	//				err = dc.FromPayloads(result, valPtr)
	//				if err != nil {
	//					panic(err)
	//				}
	//				resp := Response{
	//					Result: *valPtr,
	//					ID:     sig.ID,
	//				}
	//				wp.queue = append(wp.queue, resp)
	//				log.Printf("ACTIVITIY DONE")
	//			}))
	//			return
	//
	//		case CompleteWorkflow:
	//			wp.server.mu.Lock()
	//			delete(wp.server.workflowSignals, wp.startID.String())
	//			delete(wp.server.workflowSignals, wp.env.WorkflowInfo().WorkflowExecution.RunID)
	//			wp.server.mu.Unlock()
	//
	//			// convert PHP answer to the payload
	//			wp.env.Complete(nil, nil)
	//			return
	//		default:
	//			log.Println("unknown case")
	//			return
	//		}
	//	}
	//}
}

func (wp *workflowProxy) handle(cmd string, id uint64, params json.RawMessage) error {
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

		wp.env.ExecuteActivity(options, wp.addCallback(func(result *commonpb.Payloads, err error) {
			if err != nil {
				wp.pushError(id, err)
			} else {
				wp.pushResult(id, result)
			}
		}))

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

		wp.env.Complete(payloads, nil)
	}

	return nil
}

func (wp *workflowProxy) StackTrace() string {
	// TODO: IDEAL - debug_stacktrace()
	return "this is ST"
}

func (wp *workflowProxy) Close() {
	// TODO: CACHE OFFLOAD
	// todo: send command
}

func (wp *workflowProxy) addCallback(callback bindings.ResultHandler) bindings.ResultHandler {
	return func(result *commonpb.Payloads, err error) {
		wp.callbacks = append(wp.callbacks, func() {
			callback(result, err)
		})
	}
}

func (wp *workflowProxy) pushCommand(name string, params interface{}) (id uint64, err error) {
	cmd := rrt.Frame{
		ID:      atomic.AddUint64(wp.seqID, 1),
		Command: name,
	}

	cmd.Params, err = json.Marshal(params)
	if err != nil {
		return 0, err
	}

	wp.queue = append(wp.queue, cmd)

	return id, nil
}

func (wp *workflowProxy) pushResult(id uint64, result *commonpb.Payloads) {
	cmd := rrt.Frame{
		ID: id,
	}

	payload := rrt.RRPayload{}
	wp.env.GetDataConverter().FromPayloads(result, &payload)

	// todo: REPACK
	cmd.Result, _ = json.Marshal(payload.Data)
	wp.queue = append(wp.queue, cmd)
}

func (wp *workflowProxy) pushError(id uint64, err error) {
	//	cmd := rrt.Frame{
	//		ID:    id,
	//		Error: err.Error(),
	//}
	log.Println("error!", err)
	//wp.queue = append(wp.queue, cmd)

}

type Wrapper struct {
	Rid      string      `json:"rid"`
	Commands []rrt.Frame `json:"commands"`
}

// Exchange commands with worker.
func (wp *workflowProxy) execute(cmd ...rrt.Frame) (result []rrt.Frame, err error) {
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

	log.Printf("send: %s", color.YellowString(string(p.Body)))

	rsp, err := wp.worker.Exec(p)
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
