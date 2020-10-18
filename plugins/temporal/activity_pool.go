package temporal

import (
	"context"
	"encoding/json"
	"github.com/spiral/roadrunner/v2"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"sync"
	"time"
)

const (
	initCmd = "{\"command\":\"GetActivityWorkers\"}"
)

// ActivityPool manages set of RR and Temporal activity workers and their cancellation contexts.
type ActivityPool struct {
	workerPool      roadrunner.Pool
	temporalWorkers []worker.Worker
}

type pipelineConfig struct {
	// Pipeline options.
	Options struct {
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
	} `json:"options"`

	// Activities declares list of available activities.
	Activities []string `json:"activities"`
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the pool.
func (act *ActivityPool) InitTemporal(ctx context.Context, temporal Temporal) error {
	result, err := act.workerPool.Exec(ctx, roadrunner.Payload{Body: []byte(initCmd), Context: nil})
	if err != nil {
		return err
	}

	config := make(map[string]pipelineConfig)
	if err := json.Unmarshal(result.Body, &config); err != nil {
		return err
	}

	act.temporalWorkers = make([]worker.Worker, 0)
	for queue, pipeline := range config {
		w, err := temporal.CreateWorker(queue, worker.Options{
			MaxConcurrentActivityExecutionSize:      pipeline.Options.MaxConcurrentActivityExecutionSize,
			WorkerActivitiesPerSecond:               pipeline.Options.WorkerActivitiesPerSecond,
			MaxConcurrentLocalActivityExecutionSize: pipeline.Options.MaxConcurrentActivityExecutionSize,
			TaskQueueActivitiesPerSecond:            pipeline.Options.TaskQueueActivitiesPerSecond,
			MaxConcurrentActivityTaskPollers:        pipeline.Options.MaxConcurrentActivityTaskPollers,
		})

		if err != nil {
			act.Destroy(ctx)
			return err
		}

		act.temporalWorkers = append(act.temporalWorkers, w)

		for _, name := range pipeline.Activities {
			w.RegisterActivityWithOptions(act.handleActivity, activity.RegisterOptions{Name: name})
		}
	}

	return nil
}

func (act *ActivityPool) Start(errChan chan error) {
	for _, w := range act.temporalWorkers {
		if err := w.Start(); err != nil {
			errChan <- err
		}
	}
}

func (act *ActivityPool) handleActivity(ctx context.Context, data RRPayload) (result RRPayload, err error) {
	payload := roadrunner.Payload{}

	payload.Context, err = json.Marshal(activity.GetInfo(ctx))
	if err != nil {
		return RRPayload{}, err
	}

	payload.Body, err = json.Marshal(data.Data)
	if err != nil {
		return RRPayload{}, err
	}

	res, err := act.workerPool.Exec(ctx, payload)
	if err != nil {
		return RRPayload{}, err
	}

	// todo: async

	// todo: what results options do we have
	// todo: make sure results are packed correctly
	if err := json.Unmarshal(res.Body, &result.Data); err != nil {
		return RRPayload{}, err
	}

	return
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the pool.
func (act *ActivityPool) Destroy(ctx context.Context) {
	wg := sync.WaitGroup{}
	for _, w := range act.temporalWorkers {
		wg.Add(1)
		go func(w worker.Worker) {
			w.Stop()
			wg.Done()
		}(w)
	}

	wg.Wait()
	act.workerPool.Destroy(ctx)
}
