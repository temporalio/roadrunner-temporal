package roadrunner_temporal

import (
	"github.com/spiral/errors"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/activity"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/workflow"
	"time"
)

const (
	// GetWorkerInfo reads worker information.
	GetWorkerInfoCommand = "GetWorkerInfo"

	// InvokeActivityCommand send to worker to invoke activity.
	InvokeActivityCommand = "InvokeActivity"

	// Commands send by the host process to the worker.
	StartWorkflowCommand   = "StartWorkflow"
	InvokeSignalCommand    = "InvokeSignal"
	InvokeQueryCommand     = "InvokeQuery"
	DestroyWorkflowCommand = "DestroyWorkflow"
	CancelWorkflowCommand  = "CancelWorkflow"
	GetStackTraceCommand   = "StackTrace"

	// Commands send by worker to host process.
	ExecuteActivityCommand           = "ExecuteActivity"
	ExecuteChildWorkflowCommand      = "ExecuteChildWorkflow"
	GetChildWorkflowExecutionCommand = "GetChildWorkflowExecution"

	NewTimerCommand         = "NewTimer"
	SideEffectCommand       = "SideEffect"
	GetVersionCommand       = "GetVersion"
	CompleteWorkflowCommand = "CompleteWorkflow"
	ContinueAsNewCommand    = "ContinueAsNew"

	// External workflows
	SignalExternalWorkflowCommand = "SignalExternalWorkflow"
	CancelExternalWorkflowCommand = "CancelExternalWorkflow"

	// Unified.
	CancelCommand = "Cancel"
)

type (
	// GetWorkerInfo reads worker information.
	GetWorkerInfo struct {
	}

	// InvokeActivity invokes activity.
	InvokeActivity struct {
		// Name defines activity name.
		Name string `json:"name"`

		// Info contains execution context.
		Info activity.Info `json:"info"`

		// HeartbeatDetails indicates that the payload also contains last heartbeat details.
		HeartbeatDetails int `json:"heartbeatDetails"`
	}

	// StartWorkflow sends worker command to start workflow.
	StartWorkflow struct {
		// Info to define workflow context.
		Info *workflow.Info `json:"info"`
	}

	// InvokeQuery invokes signal with a set of arguments.
	InvokeSignal struct {
		// RunID workflow run id.
		RunID string `json:"runId"`
		// Name of the signal.
		Name string `json:"name"`
	}

	// InvokeQuery invokes query with a set of arguments.
	InvokeQuery struct {
		// RunID workflow run id.
		RunID string `json:"runId"`
		// Name of the query.
		Name string `json:"name"`
	}

	// CancelWorkflow asks worker to gracefully stop workflow, if possible (signal).
	CancelWorkflow struct {
		// RunID workflow run id.
		RunID string `json:"runId"`
	}

	// DestroyWorkflow asks worker to offload workflow from memory.
	DestroyWorkflow struct {
		// RunID workflow run id.
		RunID string `json:"runId"`
	}

	// GetStackTrace asks worker to offload workflow from memory.
	GetStackTrace struct {
		// RunID workflow run id.
		RunID string `json:"runId"`
	}

	// ExecuteActivity command by workflow worker.
	ExecuteActivity struct {
		// Name defines activity name.
		Name string `json:"name"`
		// Options to run activity.
		Options bindings.ExecuteActivityOptions `json:"options,omitempty"`
	}

	// ExecuteChildWorkflow executes child workflow.
	ExecuteChildWorkflow struct {
		// Name defines workflow name.
		Name string `json:"name"`
		// Options to run activity.
		Options bindings.WorkflowOptions `json:"options,omitempty"`
	}

	// GetChildWorkflowExecution returns the WorkflowID and RunId of child workflow.
	GetChildWorkflowExecution struct {
		// ID of child workflow command.
		ID uint64 `json:"id"`
	}

	// NewTimer starts new timer.
	NewTimer struct {
		// Milliseconds defines timer duration.
		Milliseconds int `json:"ms"`
	}

	// SideEffect to be recorded into the history.
	SideEffect struct{}

	// NewTimer starts new timer.
	GetVersion struct {
		ChangeID     string `json:"changeID"`
		MinSupported int    `json:"minSupported"`
		MaxSupported int    `json:"maxSupported"`
	}

	// CompleteWorkflow sent by worker to complete workflow. Might include additional error as part of the payload.
	CompleteWorkflow struct{}

	// ContinueAsNew restarts workflow with new running instance.
	ContinueAsNew struct {
		// Result defines workflow execution result.
		Name string `json:"name"`

		// Options for continued as new workflow.
		Options struct {
			TaskQueueName            string
			WorkflowExecutionTimeout time.Duration
			WorkflowRunTimeout       time.Duration
			WorkflowTaskTimeout      time.Duration
		} `json:"options"`
	}

	// SignalExternalWorkflow sends signal to external workflow.
	SignalExternalWorkflow struct {
		Namespace         string `json:"namespace"`
		WorkflowID        string `json:"workflowID"`
		RunID             string `json:"runID"`
		Signal            string `json:"signal"`
		ChildWorkflowOnly bool   `json:"childWorkflowOnly"`
	}

	// CancelExternalWorkflow canceller external workflow.
	CancelExternalWorkflow struct {
		Namespace  string `json:"namespace"`
		WorkflowID string `json:"workflowID"`
		RunID      string `json:"runID"`
	}

	// Cancel one or multiple internal promises (activities, local activities, timers, child workflows).
	Cancel struct {
		// CommandIDs to be cancelled.
		CommandIDs []uint64 `json:"ids"`
	}
)

// ActivityParams maps activity command to activity params.
func (cmd ExecuteActivity) ActivityParams(
	env bindings.WorkflowEnvironment,
	payloads *commonpb.Payloads,
) bindings.ExecuteActivityParams {
	params := bindings.ExecuteActivityParams{
		ExecuteActivityOptions: cmd.Options,
		ActivityType:           bindings.ActivityType{Name: cmd.Name},
		Input:                  payloads,
	}

	if params.TaskQueueName == "" {
		params.TaskQueueName = env.WorkflowInfo().TaskQueueName
	}

	return params
}

// ActivityParams maps activity command to activity params.
func (cmd ExecuteChildWorkflow) WorkflowParams(
	env bindings.WorkflowEnvironment,
	payloads *commonpb.Payloads,
) bindings.ExecuteWorkflowParams {
	params := bindings.ExecuteWorkflowParams{
		WorkflowOptions: cmd.Options,
		WorkflowType:    &bindings.WorkflowType{Name: cmd.Name},
		Input:           payloads,
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

// returns command name (only for the commands sent to the worker)
func commandName(cmd interface{}) (string, error) {
	switch cmd.(type) {
	case GetWorkerInfo, *GetWorkerInfo:
		return GetWorkerInfoCommand, nil
	case StartWorkflow, *StartWorkflow:
		return StartWorkflowCommand, nil
	case InvokeSignal, *InvokeSignal:
		return InvokeSignalCommand, nil
	case InvokeQuery, *InvokeQuery:
		return InvokeQueryCommand, nil
	case DestroyWorkflow, *DestroyWorkflow:
		return DestroyWorkflowCommand, nil
	case CancelWorkflow, *CancelWorkflow:
		return CancelWorkflowCommand, nil
	case GetStackTrace, *GetStackTrace:
		return GetStackTraceCommand, nil
	case InvokeActivity, *InvokeActivity:
		return InvokeActivityCommand, nil
	case ExecuteActivity, *ExecuteActivity:
		return ExecuteActivityCommand, nil
	case ExecuteChildWorkflow, *ExecuteChildWorkflow:
		return ExecuteChildWorkflowCommand, nil
	case GetChildWorkflowExecution, *GetChildWorkflowExecution:
		return GetChildWorkflowExecutionCommand, nil
	case NewTimer, *NewTimer:
		return NewTimerCommand, nil
	case GetVersion, *GetVersion:
		return GetVersionCommand, nil
	case SideEffect, *SideEffect:
		return SideEffectCommand, nil
	case CompleteWorkflow, *CompleteWorkflow:
		return CompleteWorkflowCommand, nil
	case ContinueAsNew, *ContinueAsNew:
		return ContinueAsNewCommand, nil
	case SignalExternalWorkflow, *SignalExternalWorkflow:
		return SignalExternalWorkflowCommand, nil
	case CancelExternalWorkflow, *CancelExternalWorkflow:
		return CancelExternalWorkflowCommand, nil
	case Cancel, *Cancel:
		return CancelCommand, nil
	default:
		return "", errors.E(errors.Op("commandName"), "undefined command type", cmd)
	}
}

// reads command from binary payload
func initCommand(name string) (interface{}, error) {
	switch name {
	case GetWorkerInfoCommand:
		return &GetWorkerInfo{}, nil

	case StartWorkflowCommand:
		return &StartWorkflow{}, nil

	case InvokeSignalCommand:
		return &InvokeSignal{}, nil

	case InvokeQueryCommand:
		return &InvokeQuery{}, nil

	case DestroyWorkflowCommand:
		return &DestroyWorkflow{}, nil

	case CancelWorkflowCommand:
		return &CancelWorkflow{}, nil

	case GetStackTraceCommand:
		return &GetStackTrace{}, nil

	case InvokeActivityCommand:
		return &InvokeActivity{}, nil

	case ExecuteActivityCommand:
		return &ExecuteActivity{}, nil

	case ExecuteChildWorkflowCommand:
		return &ExecuteChildWorkflow{}, nil

	case GetChildWorkflowExecutionCommand:
		return &GetChildWorkflowExecution{}, nil

	case NewTimerCommand:
		return &NewTimer{}, nil

	case GetVersionCommand:
		return &GetVersion{}, nil

	case SideEffectCommand:
		return &SideEffect{}, nil

	case CompleteWorkflowCommand:
		return &CompleteWorkflow{}, nil

	case ContinueAsNewCommand:
		return &ContinueAsNew{}, nil

	case SignalExternalWorkflowCommand:
		return &SignalExternalWorkflow{}, nil

	case CancelExternalWorkflowCommand:
		return &CancelExternalWorkflow{}, nil

	case CancelCommand:
		return &Cancel{}, nil

	default:
		return nil, errors.E(errors.Op("initCommand"), "undefined command type", name)
	}
}
