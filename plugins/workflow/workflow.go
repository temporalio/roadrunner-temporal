package workflow

import (
	"encoding/json"
	"github.com/spiral/roadrunner/v2"
	rrt "github.com/temporalio/roadrunner-temporal"
	commonpb "go.temporal.io/api/common/v1"
	bindings "go.temporal.io/sdk/internalbindings"
)

type workflowProxy struct {
	seqID     *uint64
	taskQueue string
	worker    roadrunner.SyncWorker
	env       bindings.WorkflowEnvironment
	queue     []interface{}
	callbacks []func()
}

func (wp *workflowProxy) Execute(env bindings.WorkflowEnvironment, header *commonpb.Header, input *commonpb.Payloads) {
	wp.callbacks = append(wp.callbacks, func() {
		//
		wp.env = env
		//
		wp.connectSignals()
		// append startWorkflow data to the command queue
		wp.startWorkflow(input)
	})
}

func (wp *workflowProxy) OnWorkflowTaskStarted() {
	//log.Print("task started")
	//defer func() {
	//	log.Println(color.YellowString("exit from OnWorkflowTaskStarted"))
	//}()
	//for _, callback := range wp.callbacks {
	//	callback()
	//}
	//wp.callbacks = nil
	//
	//wp.server.sendPayload(wp.queue) // and wait response here
	//wp.queue = nil
	//
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

func (wp *workflowProxy) connectSignals() {
	//log.Println("connecting serverSignals")
	//wp.workflowSignals = make(chan signal)
	//
	//wp.server.mu.Lock()
	//wp.server.workflowSignals[wp.env.WorkflowInfo().WorkflowExecution.RunID] = wp.workflowSignals
	//wp.server.mu.Unlock()
}

func (wp *workflowProxy) startWorkflow(input *commonpb.Payloads) {
	//info := wp.env.WorkflowInfo()
	//
	//swf := StartWorkflowStr{
	//	Method: StartWorkflow,
	//	Params: struct {
	//		Name      string             `json:"name"`
	//		Wid       string             `json:"wid"`
	//		Rid       string             `json:"rid"`
	//		TaskQueue string             `json:"taskQueue"`
	//		Payload   *commonpb.Payloads `json:"payload"`
	//	}{
	//		Name:      info.WorkflowType.Name,
	//		Wid:       info.WorkflowExecution.ID,
	//		Rid:       info.WorkflowExecution.RunID,
	//		TaskQueue: info.TaskQueueName,
	//		Payload:   input, // todo: implement
	//	},
	//	ID: wp.server.nextID(),
	//}
	//
	//wp.startID = swf.ID
	//
	//wp.server.mu.Lock()
	//wp.server.workflowSignals[string(wp.startID)] = wp.workflowSignals
	//wp.server.mu.Unlock()
	//
	//wp.queue = append(wp.queue, swf)
}

// Exchange commands with worker.
func (wp *workflowProxy) execute(cmd ...rrt.Frame) (result []rrt.Frame, err error) {
	ctx := rrt.Context{
		TaskQueue: wp.taskQueue,
		Replay:    wp.env.IsReplaying(),
		TickTime:  wp.env.Now(),
	}

	p := roadrunner.Payload{}
	if p.Context, err = json.Marshal(ctx); err != nil {
		return nil, err
	}

	p.Body, err = json.Marshal(cmd)
	if err != nil {
		return nil, err
	}

	rsp, err := wp.worker.Exec(p)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(rsp.Body, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
