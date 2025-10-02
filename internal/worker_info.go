package internal

import (
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// WorkerInfo outlines information about every available worker and it's TaskQueues.

// WorkerInfo lists available task queues, workflows and activities.
type WorkerInfo struct {
	// TaskQueue assigned to the worker.
	TaskQueue string `json:"TaskQueue"`
	// Options describe worker options.
	Options worker.Options `json:"options"`
	// PhpSdkVersion is the underlying PHP-SDK version
	PhpSdkVersion string `json:"PhpSdkVersion,omitempty"`
	// Flags are internal worker flags.
	Flags map[string]string `json:"Flags,omitempty"`
	// Workflows provided by the worker.
	Workflows []WorkflowInfo
	// Activities provided by the worker.
	Activities []ActivityInfo
}

// WorkflowInfo describes a single worker workflow.
type WorkflowInfo struct {
	// Name of the workflow.
	Name string `json:"name"`
	// Queries pre-defined for the workflow type.
	Queries []string `json:"queries"`
	// Signals pre-defined for the workflow type.
	Signals []string `json:"signals"`
	// VersioningBehavior for the workflow.
	VersioningBehavior workflow.VersioningBehavior `json:"versioning_behavior,omitempty"`
}

// ActivityInfo describes single worker activity.
type ActivityInfo struct {
	// Name describes public activity name.
	Name string `json:"name"`
}
