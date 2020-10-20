package activity_server

import (
	"context"
	"encoding/json"
	"time"

	"github.com/spiral/roadrunner/v2"
	"github.com/temporalio/roadrunner-temporal"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
)

const (
	initCmd = "{\"command\":\"GetActivityWorkers\"}"
)

var EmptyRrResult = roadrunner_temporal.RRPayload{}

type ActivityPool interface {
	InitTemporal(ctx context.Context, temporal temporal.Temporal) error
	Start() error
	Destroy(ctx context.Context)
}

// ActivityPoolImpl manages set of RR and Temporal activity workers and their cancellation contexts.
type ActivityPoolImpl struct {
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

// NewActivityPool
func NewActivityPool(pool roadrunner.Pool) ActivityPool {
	return &ActivityPoolImpl{
		workerPool: pool,
	}
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the pool.
func (act *ActivityPoolImpl) InitTemporal(ctx context.Context, temporal temporal.Temporal) error {
	result, err := act.workerPool.ExecWithContext(ctx, roadrunner.Payload{Body: []byte(initCmd), Context: nil})
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

func (act *ActivityPoolImpl) Start() error {
	for i := 0; i < len(act.temporalWorkers); i++ {
		err := act.temporalWorkers[i].Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (act *ActivityPoolImpl) handleActivity(ctx context.Context, data roadrunner_temporal.RRPayload) (roadrunner_temporal.RRPayload, error) {
	var err error
	payload := roadrunner.Payload{}

	payload.Context, err = json.Marshal(activity.GetInfo(ctx))
	if err != nil {
		return EmptyRrResult, err
	}

	payload.Body, err = json.Marshal(data.Data)
	if err != nil {
		return EmptyRrResult, err
	}

	res, err := act.workerPool.ExecWithContext(ctx, payload)
	if err != nil {
		return EmptyRrResult, err
	}

	// todo: async
	// todo: what results options do we have
	// todo: make sure results are packed correctly
	result := roadrunner_temporal.RRPayload{}
	err = json.Unmarshal(res.Body, &result.Data)
	if err != nil {
		return EmptyRrResult, err
	}

	return result, nil
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the pool.
func (act *ActivityPoolImpl) Destroy(ctx context.Context) {
	for i := 0; i < len(act.temporalWorkers); i++ {
		act.temporalWorkers[i].Stop()
	}

	// TODO add ctx.Done in RR for timeouts
	act.workerPool.Destroy(ctx)
}
