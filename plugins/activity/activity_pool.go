package activity_server

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

// activityPool manages set of RR and Temporal activity workers and their cancellation contexts.
type activityPool struct {
	workerPool      roadrunner.Pool
	temporalWorkers []worker.Worker
}

// newActivityPool
func newActivityPool(pool roadrunner.Pool) *activityPool {
	return &activityPool{
		workerPool: pool,
	}
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the pool.
func (act *activityPool) InitTemporal(ctx context.Context, temporal temporal.Temporal) error {
	result, err := act.workerPool.ExecWithContext(ctx, rrt.WorkerInit)
	if err != nil {
		return err
	}

	var info rrt.WorkerInfo
	if err := json.Unmarshal(result.Body, &info); err != nil {
		return err
	}

	act.temporalWorkers = make([]worker.Worker, 0)
	for _, cfg := range info {
		w, err := temporal.CreateWorker(cfg.TaskQueue, cfg.Options.ToTemporalOptions())

		if err != nil {
			act.Destroy(ctx)
			return err
		}

		act.temporalWorkers = append(act.temporalWorkers, w)
		for _, aCfg := range cfg.Activities {
			w.RegisterActivityWithOptions(act.handleActivity, activity.RegisterOptions{Name: aCfg.Name})
		}
	}

	return nil
}

func (act *activityPool) Start() error {
	for i := 0; i < len(act.temporalWorkers); i++ {
		err := act.temporalWorkers[i].Start()
		if err != nil {
			return err
		}
	}
	return nil
}

// todo: improve this method handling, introduce proper marshalling and faster decoding.
// todo: wrap execution into command.
func (act *activityPool) handleActivity(ctx context.Context, data rrt.RRPayload) (rrt.RRPayload, error) {
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
	result := rrt.RRPayload{}
	err = json.Unmarshal(res.Body, &result.Data)
	if err != nil {
		return EmptyRrResult, err
	}

	return result, nil
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the pool.
func (act *activityPool) Destroy(ctx context.Context) {
	for i := 0; i < len(act.temporalWorkers); i++ {
		act.temporalWorkers[i].Stop()
	}

	// TODO add ctx.Done in RR for timeouts
	act.workerPool.Destroy(ctx)
}
