package internal

import (
	"go.temporal.io/sdk/worker"
)

// WorkerInfo outlines information about every available worker and it's TaskQueues.

// WorkerInfo lists available task queues, workflows and activities.
type WorkerInfo struct {
	// TaskQueue assigned to the worker.
	TaskQueue string `json:"TaskQueue"`
	// Options describe worker options.
	Options worker.Options `json:"options,omitempty"`
	// PhpSdkVersion is the underlying PHP-SDK version
	PhpSdkVersion string `json:"PhpSdkVersion,omitempty"`
	// Flags are internal worker flags.
	Flags map[string]string `json:"Flags,omitempty"`
	// Workflows provided by the worker.
	Workflows []WorkflowInfo
	// Activities provided by the worker.
	Activities []ActivityInfo
}

// VersioningBehavior specifies when existing workflows could change their Build ID.
//
// NOTE: Experimental
//
// Exposed as: [go.temporal.io/sdk/workflow.VersioningBehavior]
type VersioningBehavior int

const (
	// VersioningBehaviorUnspecified - Workflow versioning policy unknown.
	//  A default [VersioningBehaviorUnspecified] policy forces
	// every workflow to explicitly set a [VersioningBehavior] different from [VersioningBehaviorUnspecified].
	//
	// Exposed as: [go.temporal.io/sdk/workflow.VersioningBehaviorUnspecified]
	VersioningBehaviorUnspecified VersioningBehavior = iota

	// VersioningBehaviorPinned - Workflow should be pinned to the current Build ID until manually moved.
	//
	// Exposed as: [go.temporal.io/sdk/workflow.VersioningBehaviorPinned]
	VersioningBehaviorPinned

	// VersioningBehaviorAutoUpgrade - Workflow automatically moves to the latest
	// version (default Build ID of the task queue) when the next task is dispatched.
	//
	// Exposed as: [go.temporal.io/sdk/workflow.VersioningBehaviorAutoUpgrade]
	VersioningBehaviorAutoUpgrade
)

// WorkflowInfo describes a single worker workflow.
type WorkflowInfo struct {
	// Name of the workflow.
	Name string `json:"name"`
	// Queries pre-defined for the workflow type.
	Queries []string `json:"queries"`
	// Signals pre-defined for the workflow type.
	Signals []string `json:"signals"`
	// VersioningBehavior for the workflow.
	VersioningBehavior VersioningBehavior `json:"versioning_behavior,omitempty"`
}

// ActivityInfo describes single worker activity.
type ActivityInfo struct {
	// Name describes public activity name.
	Name string `json:"name"`
}
