package temporal

import (
	"context"
	"errors"
	commonpb "go.temporal.io/api/common/v1"
	"time"

	"go.temporal.io/sdk/client"
)

/*
RecordActivityHeartbeat(ctx context.Context, taskToken []byte, details ...interface{}) error
RecordActivityHeartbeatByID(ctx context.Context, namespace, WorkflowID, runID, activityID string, details ...interface{}) error
ListClosedWorkflow(ctx context.Context, request *workflowservice.ListClosedWorkflowExecutionsRequest) (*workflowservice.ListClosedWorkflowExecutionsResponse, error)
ListOpenWorkflow(ctx context.Context, request *workflowservice.ListOpenWorkflowExecutionsRequest) (*workflowservice.ListOpenWorkflowExecutionsResponse, error)
ListWorkflow(ctx context.Context, request *workflowservice.ListWorkflowExecutionsRequest) (*workflowservice.ListWorkflowExecutionsResponse, error)
ListArchivedWorkflow(ctx context.Context, request *workflowservice.ListArchivedWorkflowExecutionsRequest) (*workflowservice.ListArchivedWorkflowExecutionsResponse, error)
ScanWorkflow(ctx context.Context, request *workflowservice.ScanWorkflowExecutionsRequest) (*workflowservice.ScanWorkflowExecutionsResponse, error)
CountWorkflow(ctx context.Context, request *workflowservice.CountWorkflowExecutionsRequest) (*workflowservice.CountWorkflowExecutionsResponse, error)
GetSearchAttributes(ctx context.Context) (*workflowservice.GetSearchAttributesResponse, error)
QueryWorkflowWithOptions(ctx context.Context, request *QueryWorkflowWithOptionsRequest) (*QueryWorkflowWithOptionsResponse, error)
DescribeTaskQueue(ctx context.Context, taskqueue string, taskqueueType enumspb.TaskQueueType) (*workflowservice.DescribeTaskQueueResponse, error)
DescribeWorkflowExecution(ctx context.Context, WorkflowID, runID string) (*workflowservice.DescribeWorkflowExecutionResponse, error)
*/

type EmptyStruct struct{}

type StartWorkflowOptions struct {
	// ID - The business identifier of the workflow execution.
	// Optional: defaulted to a uuid.
	ID string `json:"id,omitempty"`

	// TaskQueue - The workflow tasks of the workflow are scheduled on the queue with this name.
	// This is also the name of the activity task queue on which activities are scheduled.
	// The workflow author can choose to override this using activity options.
	// Mandatory: No default.
	TaskQueue string `json:"taskQueue"`

	// WorkflowExecutionTimeout - The timeout for duration of workflow execution.
	// It includes retries and continue as new. Use WorkflowRunTimeout to limit execution time
	// of a single workflow run.
	// The resolution is seconds.
	// Optional: defaulted to 10 years.
	// WorkflowExecutionTimeout time.Duration `json:"workflowExecutionTimeout"`

	// WorkflowRunTimeout - The timeout for duration of a single workflow run.
	// The resolution is seconds.
	// Optional: defaulted to WorkflowExecutionTimeout.
	// WorkflowRunTimeout time.Duration `json:"workflowRunTimeout"`

	// WorkflowTaskTimeout - The timeout for processing workflow task from the time the worker
	// pulled this task. If a workflow task is lost, it is retried after this timeout.
	// The resolution is seconds.
	// Optional: defaulted to 10 secs.
	// WorkflowTaskTimeout time.Duration `json:"workflowTaskTimeout"`
}

/*
- the method's type is exported.
- the method is exported.
- the method has two arguments, both exported (or builtin) types.
- the method's second argument is a pointer.
- the method has return type error.
*/
type rpc struct {
	srv *Plugin
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
	Name    string               `json:"name"`
	Input   []interface{}        `json:"input"`
	Options StartWorkflowOptions `json:"options"`
}

type ExecuteWorkflowOut struct {
	WorkflowID    string `json:"id"`
	WorkflowRunID string `json:"runId"`
}

func (r *rpc) ExecuteWorkflow(in ExecuteWorkflowIn, out *ExecuteWorkflowOut) error {
	ctx := context.Background()

	wr, err := r.srv.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       in.Options.ID,
		TaskQueue:                in.Options.TaskQueue,
		WorkflowExecutionTimeout: time.Minute * 10, // in.Info.WorkflowExecutionTimeout,
		WorkflowRunTimeout:       time.Minute * 10, // in.Info.WorkflowRunTimeout,
		WorkflowTaskTimeout:      time.Minute * 2,  // in.Info.WorkflowTaskTimeout,
	}, in.Name, in.Input...)
	if err != nil {
		return err
	}

	out.WorkflowID = wr.GetID()
	out.WorkflowRunID = wr.GetRunID()

	return nil
}

type GetWorkflowIn struct {
	WorkflowID    string `json:"wid"`
	WorkflowRunID string `json:"rid"`
}

type GetWorkflowResult struct {
	WorkflowID    string `json:"wid"`
	WorkflowRunID string `json:"rid,omitempty"`
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
func (r *rpc) GetWorkflow(in GetWorkflowIn, out *GetWorkflowResult) error {
	ctx := context.Background()
	wr := r.srv.client.GetWorkflow(ctx, in.WorkflowID, in.WorkflowRunID)

	out.WorkflowRunID = wr.GetRunID()
	out.WorkflowID = wr.GetID()
	return nil
}

type SignalWorkflowIn struct {
	WorkflowID    string      `json:"wid"`
	WorkflowRunID string      `json:"rid,omitempty"`
	SignalName    string      `json:"signal_name"`
	Args          interface{} `json:"args"`
}

// SignalWorkflow sends a signals to a workflow in execution
// - workflow ID of the workflow.
// - runID can be default(empс ty string). if empty string then it will pick the running execution of that workflow ID.
// - signalName name to identify the signal.
// The errors it can return:
//	- EntityNotExistsError
//	- InternalServiceError
func (r *rpc) SignalWorkflow(in SignalWorkflowIn, _ *EmptyStruct) error {
	ctx := context.Background()
	err := r.srv.client.SignalWorkflow(ctx, in.WorkflowID, in.WorkflowRunID, in.SignalName, in.Args)
	if err != nil {
		return err
	}
	return nil
}

type SignalWithStartIn struct {
	WorkflowID        string               `json:"wid"`
	SignalName        string               `json:"signal_name"`
	SignalArg         interface{}          `json:"signal_arg"`
	Options           StartWorkflowOptions `json:"options"`
	WorkflowInterface string               `json:"workflow_interface"`
	Args              []interface{}        `json:"args"`
}

type SignalWithStartOut struct {
	WorkflowID    string `json:"wid"`
	WorkflowRunID string `json:"rid"`
}

// SignalWithStartWorkflow sends a signal to a running workflow.
// If the workflow is not running or not found, it starts the workflow and then sends the signal in transaction.
// - WorkflowID, signalName, signalArg are same as SignalWorkflow's parameters
// - options, workflow, workflowArgs are same as StartWorkflow's parameters
// Note: options.WorkflowIDReusePolicy is default to AllowDuplicate in this API.
// The errors it can return:
//  - EntityNotExistsError, if namespace does not exist
//  - BadRequestError
//	- InternalServiceError
func (r *rpc) SignalWithStartWorkflow(in SignalWithStartIn, out *SignalWithStartOut) error {
	ctx := context.Background()
	wr, err := r.srv.client.SignalWithStartWorkflow(ctx, in.WorkflowID, in.SignalName, in.SignalArg, client.StartWorkflowOptions{
		ID:                       in.Options.ID,
		TaskQueue:                in.Options.TaskQueue,
		WorkflowExecutionTimeout: time.Minute, // in.Info.WorkflowExecutionTimeout,
		WorkflowRunTimeout:       time.Minute, // in.Info.WorkflowRunTimeout,
		WorkflowTaskTimeout:      time.Minute, // in.Info.WorkflowTaskTimeout,
	}, in.WorkflowInterface, in.Args...)
	if err != nil {
		return err
	}

	out.WorkflowID = wr.GetID()
	out.WorkflowRunID = wr.GetRunID()
	return nil
}

type CancelWorkflowIn struct {
	WorkflowID    string `json:"wid"`
	WorkflowRunID string `json:"rid"`
}

// CancelWorkflow request cancellation of a workflow in execution. Cancellation request closes the channel
// returned by the workflow.Context.Done() of the workflow that is target of the request.
// - workflow ID of the workflow.
// - runID can be default(empty string). if empty string then it will pick the currently running execution of that workflow ID.
// The errors it can return:
//	- EntityNotExistsError
//	- BadRequestError
//	- InternalServiceError
func (r *rpc) CancelWorkflow(in CancelWorkflowIn, _ *EmptyStruct) error {
	ctx := context.Background()
	err := r.srv.client.CancelWorkflow(ctx, in.WorkflowID, in.WorkflowRunID)
	if err != nil {
		return err
	}

	return nil
}

// TerminateWorkflow terminates a workflow execution. Terminate stops a workflow execution immediately without
// letting the workflow to perform any cleanup
// WorkflowID is required, other parameters are optional.
// - workflow ID of the workflow.
// - runID can be default(empty string). if empty string then it will pick the running execution of that workflow ID.
// The errors it can return:
//	- EntityNotExistsError
//	- BadRequestError
//	- InternalServiceError
type TerminateWorkflowIn struct {
	WorkflowID    string        `json:"wid"`
	WorkflowRunID string        `json:"rid"`
	Reason        string        `json:"reason"`
	Details       []interface{} `json:"details"`
}

func (r *rpc) TerminateWorkflow(in TerminateWorkflowIn, _ *EmptyStruct) error {
	ctx := context.Background()
	err := r.srv.client.TerminateWorkflow(ctx, in.WorkflowID, in.WorkflowRunID, in.Reason, in.Details...)
	if err != nil {
		return err
	}

	return nil
}

type CompleteActivityIn struct {
	TaskToken []byte      `json:"taskToken"`
	Result    interface{} `json:"result"`
	Error     string      `json:"error,omitempty"`
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
func (r *rpc) CompleteActivity(in CompleteActivityIn, out *CompleteActivityOut) error {
	ctx := context.Background()

	var err error
	var res interface{}

	if in.Error != "" {
		// complete with error
		err = r.srv.client.CompleteActivity(ctx, in.TaskToken, &res, errors.New(in.Error))
		if err != nil {
			return err
		}

		out.Result = nil
		return nil
	}

	// just complete
	err = r.srv.client.CompleteActivity(ctx, in.TaskToken, in.Result, nil)
	if err != nil {
		return err
	}

	out.Result = in.Result
	return nil
}

type CompleteActivityByIDIn struct {
	Namespace     string `json:"namespace"`
	WorkflowID    string `json:"wid"`
	WorkflowRunID string `json:"rid"`
	ActivityID    string `json:"activity_id"`
	Err           string `json:"err,omitempty"`
}

type CompleteActivityByIDOut struct {
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
// namespace name, WorkflowID, activityID are required, runID is optional.
// The errors it can return:
//  - ErrorWithDetails
//  - TimeoutError
//  - CanceledError
func (r *rpc) CompleteActivityByID(in CompleteActivityByIDIn, out *CompleteActivityByIDOut) error {
	ctx := context.Background()
	var res interface{}
	var err error

	if in.Err != "" {
		// complete by id with error
		err = r.srv.client.CompleteActivityByID(ctx, in.Namespace, in.WorkflowID, in.WorkflowRunID, in.ActivityID, &res, errors.New(in.Err))
		if err != nil {
			return err
		}
		out.Result = res
		return nil
	}

	// complete without error
	err = r.srv.client.CompleteActivityByID(ctx, in.Namespace, in.WorkflowID, in.WorkflowRunID, in.ActivityID, &res, nil)
	if err != nil {
		return err
	}
	out.Result = res
	return nil
}

type QueryWorkflowIn struct {
	WorkflowID    string        `json:"wid"`
	WorkflowRunID string        `json:"rid"`
	QueryType     string        `json:"query_type"`
	Args          []interface{} `json:"args"`
}

// QueryWorkflow queries a given workflow's last execution and returns the query result synchronously. Parameter WorkflowID
// and queryType are required, other parameters are optional. The WorkflowID and runID (optional) identify the
// target workflow execution that this query will be send to. If runID is not specified (empty string), server will
// use the currently running execution of that WorkflowID. The queryType specifies the type of query you want to
// run. By default, temporal supports "__stack_trace" as a standard query type, which will return string value
// representing the call stack of the target workflow. The target workflow could also setup different query handler
// to handle custom query types.
// See comments at workflow.SetQueryHandler(ctx Context, queryType string, handler interface{}) for more details
// on how to setup query handler within the target workflow.
// - WorkflowID is required.
// - runID can be default(empty string). if empty string then it will pick the running execution of that workflow ID.
// - queryType is the type of the query.
// - args... are the optional query parameters.
// The errors it can return:
//  - BadRequestError
//  - InternalServiceError
//  - EntityNotExistError
//  - QueryFailError
func (r *rpc) QueryWorkflow(in QueryWorkflowIn, out *interface{}) error {
	ctx := context.Background()
	ev, err := r.srv.client.QueryWorkflow(ctx, in.WorkflowID, in.WorkflowRunID, in.QueryType, in.Args...)
	if err != nil {
		return err
	}

	raw := &commonpb.Payloads{} // init and clear
	err = ev.Get(&raw)
	if err != nil {
		return err
	}

	// todo: fix it
	*out = raw.Payloads[0].Data

	return nil
}

type RecordActivityHeartbeatIn struct {
	TaskToken []byte
	Details   interface{}
}

// RecordActivityHeartbeat records heartbeat for an activity.
// taskToken - is the value of the binary "TaskToken" field of the "ActivityInfo" struct retrieved inside the activity.
// details - is the progress you want to record along with heart beat for this activity.
// The errors it can return:
//	- EntityNotExistsError
//	- InternalServiceError
func (r *rpc) RecordActivityHeartbeat(in RecordActivityHeartbeatIn, out *bool) error {
	ctx := context.Background()

	// todo: use proper data converter type
	err := r.srv.client.RecordActivityHeartbeat(ctx, in.TaskToken, in.Details)
	if err != nil {
		return err
	}

	*out = true

	return nil
}
