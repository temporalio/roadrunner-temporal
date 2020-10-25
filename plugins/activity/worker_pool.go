package activity

import (
	"context"
	"encoding/json"
	"github.com/spiral/roadrunner/v2"
	rrt "github.com/temporalio/roadrunner-temporal"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
)

// todo: improve it to push directly into converter
var EmptyRrResult = rrt.RRPayload{}

// workerPool manages set of RR and Temporal activity workers and their cancellation contexts.
type workerPool struct {
	workerPool      roadrunner.Pool
	temporalWorkers []worker.Worker
}

// newActivityPool
func newActivityPool(pool roadrunner.Pool) *workerPool {
	return &workerPool{
		workerPool: pool,
	}
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the pool.
func (wp *workerPool) InitPool(ctx context.Context, temporal temporal.Temporal) error {
	info, err := rrt.GetWorkerInfo(ctx, wp.workerPool)
	if err != nil {
		return err
	}

	wp.temporalWorkers = make([]worker.Worker, 0)
	for _, cfg := range info {
		w, err := temporal.CreateWorker(cfg.TaskQueue, cfg.Options.ToNativeOptions())

		if err != nil {
			wp.Destroy(ctx)
			return err
		}

		wp.temporalWorkers = append(wp.temporalWorkers, w)
		for _, aCfg := range cfg.Activities {
			w.RegisterActivityWithOptions(wp.handleActivity, activity.RegisterOptions{Name: aCfg.Name})
		}
	}

	return nil
}

func (wp *workerPool) Start() error {
	for i := 0; i < len(wp.temporalWorkers); i++ {
		err := wp.temporalWorkers[i].Start()
		if err != nil {
			return err
		}
	}
	return nil
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the pool.
func (wp *workerPool) Destroy(ctx context.Context) {
	for i := 0; i < len(wp.temporalWorkers); i++ {
		wp.temporalWorkers[i].Stop()
	}

	// TODO add ctx.Done in RR for timeouts
	wp.workerPool.Destroy(ctx)
}

// todo: improve this method handling, introduce proper marshalling and faster decoding.
// todo: wrap execution into command.
func (wp *workerPool) handleActivity(ctx context.Context, data rrt.RRPayload) (rrt.RRPayload, error) {
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

	res, err := wp.workerPool.ExecWithContext(ctx, payload)
	if err != nil {
		return EmptyRrResult, err
	}

	// todo: async
	// todo: what results options do we have
	// todo: make sure results are packed correctly
	result := rrt.RRPayload{}
	err = json.Unmarshal(res.Body, &result.Data)
	if err != nil {
		return EmptyRrResult, err
	}

	return result, nil
}
