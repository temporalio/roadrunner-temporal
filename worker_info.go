package roadrunner_temporal

import (
	"encoding/json"
	"time"

	"github.com/spiral/errors"
	"go.temporal.io/sdk/worker"
)

const getWorkerInfo = "GetWorkerInfo"

// WorkerInfo outlines information about every available worker and it's TaskQueues.
type WorkerInfo struct {
	// TaskQueue assigned to the worker.
	TaskQueue string `json:"taskQueue"`

	// Options describe worker options.
	Options WorkerOptions `json:"options,omitempty"`

	// Workflows provided by the worker.
	Workflows []WorkflowInfo

	// Activities provided by the worker.
	Activities []ActivityInfo
}

// WorkflowInfo describes single worker workflow.
type WorkflowInfo struct {
	// Name of the workflow.
	Name string `json:"name"`

	// Queries pre-defined for the workflow type.
	Queries []string `json:"queries"`

	// Signals pre-defined for the workflow type.
	Signals []string `json:"signals"`
}

// ActivityInfo describes single worker activity.
type ActivityInfo struct {
	// Name describes public activity name.
	Name string `json:"name"`
}

// WorkerOptions defined by the underlying worker. @todo: finish mapping
type WorkerOptions struct {
	// Optional: To set the maximum concurrent activity executions this worker can have.
	// The zero value of this uses the default value.
	// default: defaultMaxConcurrentActivityExecutionSize(1k)
	MaxConcurrentActivityExecutionSize int `json:"maxConcurrentActivityExecutionSize"`

	// Optional: Sets the rate limiting on number of activities that can be executed per second per
	// worker. This can be used to limit resources used by the worker.
	// Notice that the number is represented in float, so that you can set it to less than
	// 1 if needed. For example, set the number to 0.1 means you want your activity to be executed
	// once for every 10 seconds. This can be used to protect down stream services from flooding.
	// The zero value of this uses the default value
	// default: 100k
	WorkerActivitiesPerSecond float64 `json:"workerActivitiesPerSecond"`

	// Optional: Sets the rate limiting on number of activities that can be executed per second.
	// This is managed by the server and controls activities per second for your entire taskqueue
	// whereas WorkerActivityTasksPerSecond controls activities only per worker.
	// Notice that the number is represented in float, so that you can set it to less than
	// 1 if needed. For example, set the number to 0.1 means you want your activity to be executed
	// once for every 10 seconds. This can be used to protect down stream services from flooding.
	// The zero value of this uses the default value.
	// default: 100k
	TaskQueueActivitiesPerSecond float64 `json:"taskQueueActivitiesPerSecond"`

	// Optional: Sets the maximum number of goroutines that will concurrently poll the
	// temporal-server to retrieve activity tasks. Changing this value will affect the
	// rate at which the worker is able to consume tasks from a task queue.
	// default: 2
	MaxConcurrentActivityTaskPollers int `json:"maxConcurrentActivityTaskPollers"`

	// Optional: worker graceful stop timeout
	// default: 0s
	WorkerStopTimeout time.Duration `json:"workerStopTimeout"`
}

// TemporalOptions converts options to the temporal worker options.
func (opt WorkerOptions) TemporalOptions() worker.Options {
	return worker.Options{
		MaxConcurrentActivityExecutionSize:      opt.MaxConcurrentActivityExecutionSize,
		WorkerActivitiesPerSecond:               opt.WorkerActivitiesPerSecond,
		MaxConcurrentLocalActivityExecutionSize: opt.MaxConcurrentActivityExecutionSize,
		TaskQueueActivitiesPerSecond:            opt.TaskQueueActivitiesPerSecond,
		MaxConcurrentActivityTaskPollers:        opt.MaxConcurrentActivityTaskPollers,
	}
}

// GetWorkerInfo fetches information about all underlying workers (can be multiplexed inside single process).
func GetWorkerInfo(e Endpoint) ([]WorkerInfo, error) {
	result, err := Execute(e, Context{}, Message{ID: 0, Command: getWorkerInfo})
	if err != nil {
		return nil, err
	}

	if len(result) != 1 {
		return nil, errors.E(errors.Op("getWorkerInfo"), "unable to read worker info")
	}

	if result[0].ID != 0 {
		return nil, errors.E(errors.Op("getWorkerInfo"), "GetWorkerInfo confirmation missing")
	}

	var info []WorkerInfo
	for _, data := range result[0].Result {
		wi := WorkerInfo{}
		if err := json.Unmarshal(data, &wi); err != nil {
			return nil, errors.E(errors.Op("getWorkerInfo"), err)
		}

		info = append(info, wi)
	}

	return info, nil
}
