package internal

import (
	"time"

	"github.com/roadrunner-server/errors"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/api/failure/v1"
	"go.temporal.io/sdk/activity"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	getWorkerInfoCommand = "GetWorkerInfo"

	invokeActivityCommand      = "InvokeActivity"
	invokeLocalActivityCommand = "InvokeLocalActivity"
	startWorkflowCommand       = "StartWorkflow"
	invokeSignalCommand        = "InvokeSignal"
	invokeQueryCommand         = "InvokeQuery"
	destroyWorkflowCommand     = "DestroyWorkflow"
	cancelWorkflowCommand      = "CancelWorkflow"
	getStackTraceCommand       = "StackTrace"

	executeActivityCommand           = "ExecuteActivity"
	executeLocalActivityCommand      = "ExecuteLocalActivity"
	executeChildWorkflowCommand      = "ExecuteChildWorkflow"
	getChildWorkflowExecutionCommand = "GetChildWorkflowExecution"

	newTimerCommand                       = "NewTimer"
	sideEffectCommand                     = "SideEffect"
	getVersionCommand                     = "GetVersion"
	completeWorkflowCommand               = "CompleteWorkflow"
	continueAsNewCommand                  = "ContinueAsNew"
	upsertWorkflowSearchAttributesCommand = "UpsertWorkflowSearchAttributes"

	signalExternalWorkflowCommand = "SignalExternalWorkflow"
	cancelExternalWorkflowCommand = "CancelExternalWorkflow"

	undefinedResponse = "UndefinedResponse"

	cancelCommand = "Cancel"
	panicCommand  = "Panic"
)

// Context provides worker information about currently. Context can be empty for server level commands.
type Context struct {
	// TaskQueue associates message batch with the specific task queue in underlying worker.
	TaskQueue string `json:"taskQueue,omitempty"`
	// TickTime associated current or historical time with message batch.
	TickTime string `json:"tickTime,omitempty"`
	// Replay indicates that current message batch is historical.
	Replay bool `json:"replay,omitempty"`
	// History
	HistoryLen int `json:"history_length,omitempty"`
}

// Message used to exchange the send commands and receive responses from underlying workers.
type Message struct {
	// ID contains ID of the command, response or error.
	ID uint64 `json:"id"`
	// Command of the message in unmarshalled form. Pointer.
	Command any `json:"command,omitempty"`
	// Failure associated with command id.
	Failure *failure.Failure `json:"failure,omitempty"`
	// Payloads contains message specific payloads in binary format.
	Payloads *commonpb.Payloads `json:"payloads,omitempty"`
	// Header
	Header *commonpb.Header `json:"header,omitempty"`
}

// IsEmpty only check if task queue set.
func (ctx Context) IsEmpty() bool {
	return ctx.TaskQueue == ""
}

// IsCommand returns true if message carries request.
func (msg *Message) IsCommand() bool {
	return msg.Command != nil
}

func (msg *Message) UndefinedResponse() bool {
	if _, ok := msg.Command.(*UndefinedResponse); ok {
		return true
	}

	return false
}

func (msg *Message) Reset() {
	msg.ID = 0
	msg.Command = nil
	msg.Failure = nil
	msg.Payloads = nil
	msg.Header = nil
}

// GetWorkerInfo reads worker information.
type GetWorkerInfo struct {
	RRVersion string `json:"rr_version"`
}

// InvokeActivity invokes activity.
type InvokeActivity struct {
	// Name defines activity name.
	Name string `json:"name"`

	// Info contains execution context.
	Info activity.Info `json:"info"`

	// HeartbeatDetails indicates that the payload also contains last heartbeat details.
	HeartbeatDetails int `json:"heartbeatDetails,omitempty"`
}

// InvokeLocalActivity invokes local activity.
type InvokeLocalActivity struct {
	// Name defines activity name.
	Name string `json:"name"`

	// Info contains execution context.
	Info activity.Info `json:"info"`
}

// StartWorkflow sends worker command to start workflow.
type StartWorkflow struct {
	// Info to define workflow context.
	Info *workflow.Info `json:"info"`

	// LastCompletion contains offset of last completion results.
	LastCompletion int `json:"lastCompletion,omitempty"`
}

// InvokeSignal invokes signal with a set of arguments.
type InvokeSignal struct {
	// RunID workflow run id.
	RunID string `json:"runId"`

	// Name of the signal.
	Name string `json:"name"`
}

// InvokeQuery invokes query with a set of arguments.
type InvokeQuery struct {
	// RunID workflow run id.
	RunID string `json:"runId"`
	// Name of the query.
	Name string `json:"name"`
}

// CancelWorkflow asks worker to gracefully stop workflow, if possible (signal).
type CancelWorkflow struct {
	// RunID workflow run id.
	RunID string `json:"runId"`
}

// DestroyWorkflow asks worker to offload workflow from memory.
type DestroyWorkflow struct {
	// RunID workflow run id.
	RunID string `json:"runId"`
}

// GetStackTrace asks worker to offload workflow from memory.
type GetStackTrace struct {
	// RunID workflow run id.
	RunID string `json:"runId"`
}

// ExecuteActivity command by workflow worker.
type ExecuteActivity struct {
	// Name defines activity name.
	Name string `json:"name"`
	// Options to run activity.
	Options bindings.ExecuteActivityOptions `json:"options,omitempty"`
}

// ExecuteLocalActivityOptions .. since we use proto everywhere we need to convert Activity options (proto) to non-proto LA options
type ExecuteLocalActivityOptions struct {
	ScheduleToCloseTimeout time.Duration
	StartToCloseTimeout    time.Duration
	RetryPolicy            *commonpb.RetryPolicy
}

// ExecuteLocalActivity command by workflow worker.
type ExecuteLocalActivity struct {
	// Name defines activity name.
	Name string `json:"name"`
	// Options to run activity.
	Options ExecuteLocalActivityOptions `json:"options,omitempty"`
}

// ExecuteChildWorkflow executes child workflow.
type ExecuteChildWorkflow struct {
	// Name defines workflow name.
	Name string `json:"name"`
	// Options to run activity.
	Options bindings.WorkflowOptions `json:"options,omitempty"`
}

// GetChildWorkflowExecution returns the WorkflowID and RunId of child workflow.
type GetChildWorkflowExecution struct {
	// ID of child workflow command.
	ID uint64 `json:"id"`
}

// NewTimer starts new timer.
type NewTimer struct {
	// Milliseconds defines timer duration.
	Milliseconds int `json:"ms"`
}

// SideEffect to be recorded into the history.
type SideEffect struct{}

// GetVersion requests version marker.
type GetVersion struct {
	ChangeID     string `json:"changeID"`
	MinSupported int    `json:"minSupported"`
	MaxSupported int    `json:"maxSupported"`
}

// CompleteWorkflow sent by worker to complete workflow. Might include additional error as part of the payload.
type CompleteWorkflow struct{}

// ContinueAsNew restarts workflow with new running instance.
type ContinueAsNew struct {
	// Result defines workflow execution result.
	Name string `json:"name"`

	// Options for continued as new workflow.
	Options struct {
		TaskQueueName       string
		WorkflowRunTimeout  time.Duration
		WorkflowTaskTimeout time.Duration
	} `json:"options"`
}

// UpsertWorkflowSearchAttributes allows to upsert search attributes
type UpsertWorkflowSearchAttributes struct {
	SearchAttributes map[string]any `json:"searchAttributes"`
}

// SignalExternalWorkflow sends signal to external workflow.
type SignalExternalWorkflow struct {
	Namespace         string `json:"namespace"`
	WorkflowID        string `json:"workflowID"`
	RunID             string `json:"runID"`
	Signal            string `json:"signal"`
	ChildWorkflowOnly bool   `json:"childWorkflowOnly"`
}

// CancelExternalWorkflow canceller external workflow.
type CancelExternalWorkflow struct {
	Namespace  string `json:"namespace"`
	WorkflowID string `json:"workflowID"`
	RunID      string `json:"runID"`
}

// UndefinedResponse indicates that we should panic the workflow
type UndefinedResponse struct {
	Message string `json:"message"`
}

// Cancel one or multiple internal promises (activities, local activities, timers, child workflows).
type Cancel struct {
	// CommandIDs to be canceled.
	CommandIDs []uint64 `json:"ids"`
}

// Panic triggers panic in workflow process.
type Panic struct {
	// Message to include into the error.
	Message string `json:"message"`
}

// ActivityParams maps activity command to activity params.
func (cmd ExecuteActivity) ActivityParams(env bindings.WorkflowEnvironment, payloads *commonpb.Payloads, header *commonpb.Header) bindings.ExecuteActivityParams {
	params := bindings.ExecuteActivityParams{
		ExecuteActivityOptions: cmd.Options,
		ActivityType:           bindings.ActivityType{Name: cmd.Name},
		Input:                  payloads,
		Header:                 header,
	}

	if params.TaskQueueName == "" {
		params.TaskQueueName = env.WorkflowInfo().TaskQueueName
	}

	return params
}

// LocalActivityParams maps activity command to activity params.
func (cmd ExecuteLocalActivity) LocalActivityParams(env bindings.WorkflowEnvironment, fn any, payloads *commonpb.Payloads, header *commonpb.Header) bindings.ExecuteLocalActivityParams {
	if cmd.Options.StartToCloseTimeout == 0 {
		cmd.Options.StartToCloseTimeout = time.Minute
	}
	if cmd.Options.ScheduleToCloseTimeout == 0 {
		cmd.Options.ScheduleToCloseTimeout = time.Minute
	}

	truTemOptions := bindings.ExecuteLocalActivityOptions{
		ScheduleToCloseTimeout: cmd.Options.ScheduleToCloseTimeout,
		StartToCloseTimeout:    cmd.Options.StartToCloseTimeout,
	}

	if cmd.Options.RetryPolicy != nil {
		rp := &temporal.RetryPolicy{
			InitialInterval:        ifNotNil(cmd.Options.RetryPolicy.InitialInterval),
			BackoffCoefficient:     cmd.Options.RetryPolicy.BackoffCoefficient,
			MaximumInterval:        ifNotNil(cmd.Options.RetryPolicy.MaximumInterval),
			MaximumAttempts:        cmd.Options.RetryPolicy.MaximumAttempts,
			NonRetryableErrorTypes: cmd.Options.RetryPolicy.NonRetryableErrorTypes,
		}

		truTemOptions.RetryPolicy = rp
	}

	params := bindings.ExecuteLocalActivityParams{
		ExecuteLocalActivityOptions: truTemOptions,
		ActivityFn:                  fn,
		ActivityType:                cmd.Name,
		InputArgs:                   []any{payloads},
		WorkflowInfo:                env.WorkflowInfo(),
		ScheduledTime:               time.Now(),
		Header:                      header,
	}

	return params
}

func ifNotNil(val *time.Duration) time.Duration {
	if val != nil {
		return *val
	}
	return 0
}

// WorkflowParams maps workflow command to workflow params.
func (cmd ExecuteChildWorkflow) WorkflowParams(env bindings.WorkflowEnvironment, payloads *commonpb.Payloads, header *commonpb.Header) bindings.ExecuteWorkflowParams {
	params := bindings.ExecuteWorkflowParams{
		WorkflowOptions: cmd.Options,
		WorkflowType:    &bindings.WorkflowType{Name: cmd.Name},
		Input:           payloads,
		Header:          header,
	}

	if params.TaskQueueName == "" {
		params.TaskQueueName = env.WorkflowInfo().TaskQueueName
	}

	return params
}

// ToDuration converts timer command to time.Duration.
func (cmd NewTimer) ToDuration() time.Duration {
	return time.Millisecond * time.Duration(cmd.Milliseconds)
}

// CommandName returns command name (only for the commands sent to the worker)
func CommandName(cmd any) (string, error) {
	const op = errors.Op("command_name")
	switch cmd.(type) {
	case GetWorkerInfo, *GetWorkerInfo:
		return getWorkerInfoCommand, nil
	case StartWorkflow, *StartWorkflow:
		return startWorkflowCommand, nil
	case InvokeSignal, *InvokeSignal:
		return invokeSignalCommand, nil
	case InvokeQuery, *InvokeQuery:
		return invokeQueryCommand, nil
	case DestroyWorkflow, *DestroyWorkflow:
		return destroyWorkflowCommand, nil
	case CancelWorkflow, *CancelWorkflow:
		return cancelWorkflowCommand, nil
	case GetStackTrace, *GetStackTrace:
		return getStackTraceCommand, nil
	case InvokeActivity, *InvokeActivity:
		return invokeActivityCommand, nil
	case ExecuteActivity, *ExecuteActivity:
		return executeActivityCommand, nil
	case InvokeLocalActivity, *InvokeLocalActivity:
		return invokeLocalActivityCommand, nil
	case ExecuteLocalActivity, *ExecuteLocalActivity:
		return executeLocalActivityCommand, nil
	case ExecuteChildWorkflow, *ExecuteChildWorkflow:
		return executeChildWorkflowCommand, nil
	case GetChildWorkflowExecution, *GetChildWorkflowExecution:
		return getChildWorkflowExecutionCommand, nil
	case NewTimer, *NewTimer:
		return newTimerCommand, nil
	case GetVersion, *GetVersion:
		return getVersionCommand, nil
	case SideEffect, *SideEffect:
		return sideEffectCommand, nil
	case CompleteWorkflow, *CompleteWorkflow:
		return completeWorkflowCommand, nil
	case ContinueAsNew, *ContinueAsNew:
		return continueAsNewCommand, nil
	case UpsertWorkflowSearchAttributes, *UpsertWorkflowSearchAttributes:
		return upsertWorkflowSearchAttributesCommand, nil
	case SignalExternalWorkflow, *SignalExternalWorkflow:
		return signalExternalWorkflowCommand, nil
	case CancelExternalWorkflow, *CancelExternalWorkflow:
		return cancelExternalWorkflowCommand, nil
	case Cancel, *Cancel:
		return cancelCommand, nil
	case Panic, *Panic:
		return panicCommand, nil
	default:
		return "", errors.E(op, errors.Errorf("undefined command type: %s", cmd))
	}
}

// InitCommand reads command from binary payload
func InitCommand(name string) (any, error) {
	const op = errors.Op("init_command")
	switch name {
	case getWorkerInfoCommand:
		return &GetWorkerInfo{}, nil

	case startWorkflowCommand:
		return &StartWorkflow{}, nil

	case invokeSignalCommand:
		return &InvokeSignal{}, nil

	case invokeQueryCommand:
		return &InvokeQuery{}, nil

	case destroyWorkflowCommand:
		return &DestroyWorkflow{}, nil

	case cancelWorkflowCommand:
		return &CancelWorkflow{}, nil

	case getStackTraceCommand:
		return &GetStackTrace{}, nil

	case invokeActivityCommand:
		return &InvokeActivity{}, nil

	case executeActivityCommand:
		return &ExecuteActivity{}, nil

	case executeLocalActivityCommand:
		return &ExecuteLocalActivity{}, nil

	case executeChildWorkflowCommand:
		return &ExecuteChildWorkflow{}, nil

	case getChildWorkflowExecutionCommand:
		return &GetChildWorkflowExecution{}, nil

	case newTimerCommand:
		return &NewTimer{}, nil

	case getVersionCommand:
		return &GetVersion{}, nil

	case sideEffectCommand:
		return &SideEffect{}, nil

	case completeWorkflowCommand:
		return &CompleteWorkflow{}, nil

	case continueAsNewCommand:
		return &ContinueAsNew{}, nil

	case upsertWorkflowSearchAttributesCommand:
		return &UpsertWorkflowSearchAttributes{}, nil

	case signalExternalWorkflowCommand:
		return &SignalExternalWorkflow{}, nil

	case cancelExternalWorkflowCommand:
		return &CancelExternalWorkflow{}, nil

	case cancelCommand:
		return &Cancel{}, nil

	case panicCommand:
		return &Panic{}, nil

	case undefinedResponse:
		return &UndefinedResponse{}, nil

	default:
		return nil, errors.E(op, errors.Errorf("undefined command name: %s, possible outdated RoadRunner version", name))
	}
}
