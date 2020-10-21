package temporal

import (
	"context"
	"errors"
	"time"

	payload "github.com/temporalio/roadrunner-temporal"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

// TODO heavy return payloads, we should figure out, what exactly do we need in response
/*
RecordActivityHeartbeat(ctx context.Context, taskToken []byte, details ...interface{}) error
RecordActivityHeartbeatByID(ctx context.Context, namespace, workflowID, runID, activityID string, details ...interface{}) error
ListClosedWorkflow(ctx context.Context, request *workflowservice.ListClosedWorkflowExecutionsRequest) (*workflowservice.ListClosedWorkflowExecutionsResponse, error)
ListOpenWorkflow(ctx context.Context, request *workflowservice.ListOpenWorkflowExecutionsRequest) (*workflowservice.ListOpenWorkflowExecutionsResponse, error)
ListWorkflow(ctx context.Context, request *workflowservice.ListWorkflowExecutionsRequest) (*workflowservice.ListWorkflowExecutionsResponse, error)
ListArchivedWorkflow(ctx context.Context, request *workflowservice.ListArchivedWorkflowExecutionsRequest) (*workflowservice.ListArchivedWorkflowExecutionsResponse, error)
ScanWorkflow(ctx context.Context, request *workflowservice.ScanWorkflowExecutionsRequest) (*workflowservice.ScanWorkflowExecutionsResponse, error)
CountWorkflow(ctx context.Context, request *workflowservice.CountWorkflowExecutionsRequest) (*workflowservice.CountWorkflowExecutionsResponse, error)
etSearchAttributes(ctx context.Context) (*workflowservice.GetSearchAttributesResponse, error)
QueryWorkflowWithOptions(ctx context.Context, request *QueryWorkflowWithOptionsRequest) (*QueryWorkflowWithOptionsResponse, error)
DescribeTaskQueue(ctx context.Context, taskqueue string, taskqueueType enumspb.TaskQueueType) (*workflowservice.DescribeTaskQueueResponse, error)
DescribeWorkflowExecution(ctx context.Context, workflowID, runID string) (*workflowservice.DescribeWorkflowExecutionResponse, error)
*/

const name = "rpc"

type EmptyStruct struct{}

type StartWorkflowOptions struct {
	// ID - The business identifier of the workflow execution.
	// Optional: defaulted to a uuid.
	ID string `json:"wid,omitempty"`

	// TaskQueue - The workflow tasks of the workflow are scheduled on the queue with this name.
	// This is also the name of the activity task queue on which activities are scheduled.
	// The workflow author can choose to override this using activity options.
	// Mandatory: No default.
	TaskQueue string `json:"task_queue"`

	// WorkflowExecutionTimeout - The timeout for duration of workflow execution.
	// It includes retries and continue as new. Use WorkflowRunTimeout to limit execution time
	// of a single workflow run.
	// The resolution is seconds.
	// Optional: defaulted to 10 years.
	WorkflowExecutionTimeout time.Duration `json:"workflow_execution_timeout"`

	// WorkflowRunTimeout - The timeout for duration of a single workflow run.
	// The resolution is seconds.
	// Optional: defaulted to WorkflowExecutionTimeout.
	WorkflowRunTimeout time.Duration `json:"workflow_run_timeout"`

	// WorkflowTaskTimeout - The timeout for processing workflow task from the time the worker
	// pulled this task. If a workflow task is lost, it is retried after this timeout.
	// The resolution is seconds.
	// Optional: defaulted to 10 secs.
	WorkflowTaskTimeout time.Duration `json:"workflow_task_timeout"`
}

/*
- the method's type is exported.
- the method is exported.
- the method has two arguments, both exported (or builtin) types.
- the method's second argument is a pointer.
- the method has return type error.
*/
type Rpc struct {
	client client.Client
}

// ExecuteWorkflow starts a workflow execution and return a WorkflowRun instance and error
// The user can use this to start using a function or workflow type name.
// Either by
//     ExecuteWorkflow(ctx, options, "workflowTypeName", arg1, arg2, arg3)
//     or
//     ExecuteWorkflow(ctx, options, workflowExecuteFn, arg1, arg2, arg3)
// The errors it can return:
//	- EntityNotExistsError, if namespace does not exists
//	- BadRequestError
//	- InternalServiceError
//
// WorkflowRun has 3 methods:
//  - GetWorkflowID() string: which return the started workflow ID
//  - GetRunID() string: which return the first started workflow run ID (please see below)
//  - Get(ctx context.Context, valuePtr interface{}) error: which will fill the workflow
//    execution result to valuePtr, if workflow execution is a success, or return corresponding
//    error. This is a blocking API.
// NOTE: if the started workflow return ContinueAsNewError during the workflow execution, the
// return result of GetRunID() will be the started workflow run ID, not the new run ID caused by ContinueAsNewError,
// however, Get(ctx context.Context, valuePtr interface{}) will return result from the run which did not return ContinueAsNewError.
// Say ExecuteWorkflow started a workflow, in its first run, has run ID "run ID 1", and returned ContinueAsNewError,
// the second run has run ID "run ID 2" and return some result other than ContinueAsNewError:
// GetRunID() will always return "run ID 1" and  Get(ctx context.Context, valuePtr interface{}) will return the result of second run.
// NOTE: DO NOT USE THIS API INSIDE A WORKFLOW, USE workflow.ExecuteChildWorkflow instead

type ExecuteWorkflowIn struct {
	Options StartWorkflowOptions `json:"options"`
}

type ExecuteWorkflowOut struct {
	WorkflowId    string `json:"wid"`
	WorkflowRunId string `json:"rid"`
}

func (r *Rpc) ExecuteWorkflow(in ExecuteWorkflowIn, out *ExecuteWorkflowOut) error {
	ctx := context.Background()
	wr, err := r.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       in.Options.ID,
		TaskQueue:                in.Options.TaskQueue,
		WorkflowExecutionTimeout: in.Options.WorkflowExecutionTimeout,
		WorkflowRunTimeout:       in.Options.WorkflowRunTimeout,
		WorkflowTaskTimeout:      in.Options.WorkflowTaskTimeout,
	}, nil, nil)
	if err != nil {
		return err
	}
	out.WorkflowId = wr.GetID()
	out.WorkflowRunId = wr.GetRunID()
	return nil
}

type GetWorkflowIn struct {
	WorkflowId    string `json:"wid"`
	WorkflowRunId string `json:"rid"`
}

type GetWorkflowResult struct {
	WorkflowId    string `json:"wid"`
	WorkflowRunId string `json:"rid,omitempty"`
}

// GetWorkflow retrieves a workflow execution and return a WorkflowRun instance (described above)
// - workflow ID of the workflow.
// - runID can be default(empty string). if empty string then it will pick the last running execution of that workflow ID.
//
// WorkflowRun has 2 methods:
//  - GetRunID() string: which return the first started workflow run ID (please see below)
//  - Get(ctx context.Context, valuePtr interface{}) error: which will fill the workflow
//    execution result to valuePtr, if workflow execution is a success, or return corresponding
//    error. This is a blocking API.
// If workflow not found, the Get() will return EntityNotExistsError.
// NOTE: if the started workflow return ContinueAsNewError during the workflow execution, the
// return result of GetRunID() will be the started workflow run ID, not the new run ID caused by ContinueAsNewError,
// however, Get(ctx context.Context, valuePtr interface{}) will return result from the run which did not return ContinueAsNewError.
// Say ExecuteWorkflow started a workflow, in its first run, has run ID "run ID 1", and returned ContinueAsNewError,
// the second run has run ID "run ID 2" and return some result other than ContinueAsNewError:
// GetRunID() will always return "run ID 1" and  Get(ctx context.Context, valuePtr interface{}) will return the result of second run.
func (r *Rpc) GetWorkflow(in GetWorkflowIn, out *GetWorkflowResult) error {
	ctx := context.Background()
	wr := r.client.GetWorkflow(ctx, in.WorkflowId, in.WorkflowRunId)

	(*out).WorkflowRunId = wr.GetRunID()
	(*out).WorkflowId = wr.GetID()
	return nil
}

type SignalWorkflowIn struct {
	WorkflowId    string        `json:"wid"`
	WorkflowRunId string        `json:"rid,omitempty"`
	SignalName    string        `json:"signal_name"`
	Args          []interface{} `json:"args"`
}

// SignalWorkflow sends a signals to a workflow in execution
// - workflow ID of the workflow.
// - runID can be default(empty string). if empty string then it will pick the running execution of that workflow ID.
// - signalName name to identify the signal.
// The errors it can return:
//	- EntityNotExistsError
//	- InternalServiceError
func (r *Rpc) SignalWorkflow(in SignalWorkflowIn, _ *EmptyStruct) error {
	ctx := context.Background()
	err := r.client.SignalWorkflow(ctx, in.WorkflowId, in.WorkflowRunId, in.SignalName, in.Args)
	if err != nil {
		return err
	}
	return nil
}

type SignalWithStartIn struct {
	WorkflowId        string               `json:"wid"`
	SignalName        string               `json:"signal_name"`
	SignalArg         interface{}          `json:"signal_arg"`
	Options           StartWorkflowOptions `json:"options"`
	WorkflowInterface string               `json:"workflow_interface"`
	Args              []interface{}        `json:"args"`
}

type SignalWithStartOut struct {
	WorkflowId    string `json:"wid"`
	WorkflowRunId string `json:"rid"`
}

// SignalWithStartWorkflow sends a signal to a running workflow.
// If the workflow is not running or not found, it starts the workflow and then sends the signal in transaction.
// - workflowID, signalName, signalArg are same as SignalWorkflow's parameters
// - options, workflow, workflowArgs are same as StartWorkflow's parameters
// Note: options.WorkflowIDReusePolicy is default to AllowDuplicate in this API.
// The errors it can return:
//  - EntityNotExistsError, if namespace does not exist
//  - BadRequestError
//	- InternalServiceError
func (r *Rpc) SignalWithStartWorkflow(in SignalWithStartIn, out *SignalWithStartOut) error {
	ctx := context.Background()
	wr, err := r.client.SignalWithStartWorkflow(ctx, in.WorkflowId, in.SignalName, in.SignalArg, client.StartWorkflowOptions{
		ID:                       in.Options.ID,
		TaskQueue:                in.Options.TaskQueue,
		WorkflowExecutionTimeout: in.Options.WorkflowExecutionTimeout,
		WorkflowRunTimeout:       in.Options.WorkflowRunTimeout,
		WorkflowTaskTimeout:      in.Options.WorkflowTaskTimeout,
	}, in.WorkflowInterface, in.Args...)
	if err != nil {
		return err
	}

	(*out).WorkflowId = wr.GetID()
	(*out).WorkflowRunId = wr.GetRunID()
	return nil
}

type CancelWorkflowIn struct {
	WorkflowId    string `json:"wid"`
	WorkflowRunId string `json:"rid"`
}

// CancelWorkflow request cancellation of a workflow in execution. Cancellation request closes the channel
// returned by the workflow.Context.Done() of the workflow that is target of the request.
// - workflow ID of the workflow.
// - runID can be default(empty string). if empty string then it will pick the currently running execution of that workflow ID.
// The errors it can return:
//	- EntityNotExistsError
//	- BadRequestError
//	- InternalServiceError
func (r *Rpc) CancelWorkflow(in CancelWorkflowIn, _ *EmptyStruct) error {
	ctx := context.Background()
	err := r.client.CancelWorkflow(ctx, in.WorkflowId, in.WorkflowRunId)
	if err != nil {
		return err
	}

	return nil
}

// TerminateWorkflow terminates a workflow execution. Terminate stops a workflow execution immediately without
// letting the workflow to perform any cleanup
// workflowID is required, other parameters are optional.
// - workflow ID of the workflow.
// - runID can be default(empty string). if empty string then it will pick the running execution of that workflow ID.
// The errors it can return:
//	- EntityNotExistsError
//	- BadRequestError
//	- InternalServiceError
type TerminateWorkflowIn struct {
	WorkflowId    string        `json:"wid"`
	WorkflowRunId string        `json:"rid"`
	Reason        string        `json:"reason"`
	Details       []interface{} `json:"details"`
}

func (r *Rpc) TerminateWorkflow(in TerminateWorkflowIn, _ *EmptyStruct) error {
	ctx := context.Background()
	err := r.client.TerminateWorkflow(ctx, in.WorkflowId, in.WorkflowRunId, in.Reason, in.Details...)
	if err != nil {
		return err
	}

	return nil
}

type HistoryEventFilterType int32

const (
	HISTORY_EVENT_FILTER_TYPE_UNSPECIFIED HistoryEventFilterType = 0
	HISTORY_EVENT_FILTER_TYPE_ALL_EVENT   HistoryEventFilterType = 1
	HISTORY_EVENT_FILTER_TYPE_CLOSE_EVENT HistoryEventFilterType = 2
)

type GetWorkflowHistoryIn struct {
	WorkflowId    string                 `json:"wid"`
	WorkflowRunId string                 `json:"rid"`
	FilterType    HistoryEventFilterType `json:"filter_type"` // int32
}

type HistoryEvent struct {
	EventId   int64  `json:"event_id"`
	EventTime string `json:"event_time"` // time in UNIX date format
	EventType string `json:"event_type"`
	TaskId    int64  `json:"task_id"`
	Version   int64  `json:"version"`
}

type GetWorkflowHistoryOut struct {
	HistoryEvents []HistoryEvent `json:"history_events"`
}

func (r *Rpc) GetWorkflowHistory(in GetWorkflowHistoryIn, out *GetWorkflowHistoryOut) error {
	if in.FilterType < 0 || in.FilterType > 2 {
		return errors.New("filter type should be between 0 and 2 inclusive")
	}
	ctx := context.Background()
	iterator := r.client.GetWorkflowHistory(ctx, in.WorkflowId, in.WorkflowRunId, false, enums.HistoryEventFilterType(in.FilterType))

	(*out).HistoryEvents = make([]HistoryEvent, 0, 10)
	for iterator.HasNext() {
		historyEvent, err := iterator.Next()
		if err != nil {
			return err
		}

		(*out).HistoryEvents = append((*out).HistoryEvents, HistoryEvent{
			EventId:   historyEvent.EventId,
			EventTime: historyEvent.EventTime.Format(time.UnixDate),
			EventType: historyEvent.EventType.String(),
			TaskId:    historyEvent.TaskId,
			Version:   historyEvent.Version,
		})
	}

	return nil
}

type CompleteActivityIn struct {
	TaskToken []byte `json:"task_token"`
	Err       string `json:"err,omitempty"`
}

type CompleteActivityOut struct {
	Result interface{} `json:"result"`
}

// CompleteActivity reports activity completed.
// activity Execute method can return activity.ErrResultPending to
// indicate the activity is not completed when it's Execute method returns. In that case, this CompleteActivity() method
// should be called when that activity is completed with the actual result and error. If err is nil, activity task
// completed event will be reported; if err is CanceledError, activity task canceled event will be reported; otherwise,
// activity task failed event will be reported.
// An activity implementation should use GetActivityInfo(ctx).TaskToken function to get task token to use for completion.
// Example:-
//	To complete with a result.
//  	CompleteActivity(token, "Done", nil)
//	To fail the activity with an error.
//      CompleteActivity(token, nil, temporal.NewApplicationError("reason", details)
// The activity can fail with below errors ErrorWithDetails, TimeoutError, CanceledError.
func (r *Rpc) CompleteActivity(in CompleteActivityIn, out *CompleteActivityOut) error {
	ctx := context.Background()
	var err error
	var res interface{}
	if in.Err != "" {
		// complete with error
		err = r.client.CompleteActivity(ctx, in.TaskToken, &res, errors.New(in.Err))
		if err != nil {
			return err
		}

		(*out).Result = res
		return nil
	}

	// just complete
	err = r.client.CompleteActivity(ctx, in.TaskToken, &res, nil)
	if err != nil {
		return err
	}

	(*out).Result = res
	return nil
}

type CompleteActivityByIdIn struct {
	Namespace     string `json:"namespace"`
	WorkflowId    string `json:"wid"`
	WorkflowRunId string `json:"rid"`
	ActivityId    string `json:"activity_id"`
	Err           string `json:"err,omitempty"`
}

type CompleteActivityByIdOut struct {
	Result interface{} `json:"result"`
}

// CompleteActivityByID reports activity completed.
// Similar to CompleteActivity, but may save user from keeping taskToken info.
// activity Execute method can return activity.ErrResultPending to
// indicate the activity is not completed when it's Execute method returns. In that case, this CompleteActivityById() method
// should be called when that activity is completed with the actual result and error. If err is nil, activity task
// completed event will be reported; if err is CanceledError, activity task canceled event will be reported; otherwise,
// activity task failed event will be reported.
// An activity implementation should use activityID provided in ActivityOption to use for completion.
// namespace name, workflowID, activityID are required, runID is optional.
// The errors it can return:
//  - ErrorWithDetails
//  - TimeoutError
//  - CanceledError
func (r *Rpc) CompleteActivityByID(in CompleteActivityByIdIn, out *CompleteActivityByIdOut) error {
	ctx := context.Background()
	var res interface{}
	var err error

	if in.Err != "" {
		// complete by id with error
		err = r.client.CompleteActivityByID(ctx, in.Namespace, in.WorkflowId, in.WorkflowRunId, in.ActivityId, &res, errors.New(in.Err))
		if err != nil {
			return err
		}
		(*out).Result = res
		return nil
	}

	// complete without error
	err = r.client.CompleteActivityByID(ctx, in.Namespace, in.WorkflowId, in.WorkflowRunId, in.ActivityId, &res, nil)
	if err != nil {
		return err
	}
	(*out).Result = res
	return nil

}

type QueryWorkflowIn struct {
	WorkflowId    string        `json:"wid"`
	WorkflowRunId string        `json:"rid"`
	QueryType     string        `json:"query_type"`
	Args          []interface{} `json:"args"`
}

// QueryWorkflow queries a given workflow's last execution and returns the query result synchronously. Parameter workflowID
// and queryType are required, other parameters are optional. The workflowID and runID (optional) identify the
// target workflow execution that this query will be send to. If runID is not specified (empty string), server will
// use the currently running execution of that workflowID. The queryType specifies the type of query you want to
// run. By default, temporal supports "__stack_trace" as a standard query type, which will return string value
// representing the call stack of the target workflow. The target workflow could also setup different query handler
// to handle custom query types.
// See comments at workflow.SetQueryHandler(ctx Context, queryType string, handler interface{}) for more details
// on how to setup query handler within the target workflow.
// - workflowID is required.
// - runID can be default(empty string). if empty string then it will pick the running execution of that workflow ID.
// - queryType is the type of the query.
// - args... are the optional query parameters.
// The errors it can return:
//  - BadRequestError
//  - InternalServiceError
//  - EntityNotExistError
//  - QueryFailError
func (r *Rpc) QueryWorkflow(in QueryWorkflowIn, out *payload.RRPayload) error {
	ctx := context.Background()
	ev, err := r.client.QueryWorkflow(ctx, in.WorkflowId, in.WorkflowRunId, in.QueryType, in.Args...)
	if err != nil {
		return err
	}

	out = &payload.RRPayload{} // init and clear
	err = ev.Get(out)
	if err != nil {
		return err
	}
	return nil
}
