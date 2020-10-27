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

// session manages set of RR and Temporal activity workers and their cancellation contexts.
type session struct {
	workerPool      roadrunner.Pool
	temporalWorkers []worker.Worker
}

// newSession
func newSession(pool roadrunner.Pool) *session {
	return &session{
		workerPool: pool,
	}
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the ss.
func (ss *session) InitPool(ctx context.Context, temporal temporal.Temporal) error {
	info, err := rrt.GetWorkerInfo(ctx, ss.workerPool)
	if err != nil {
		return err
	}

	ss.temporalWorkers = make([]worker.Worker, 0)
	for _, cfg := range info {
		w, err := temporal.CreateWorker(cfg.TaskQueue, cfg.Options.TemporalOptions())

		if err != nil {
			ss.Destroy(ctx)
			return err
		}

		ss.temporalWorkers = append(ss.temporalWorkers, w)
		for _, aCfg := range cfg.Activities {
			w.RegisterActivityWithOptions(ss.handleActivity, activity.RegisterOptions{Name: aCfg.Name})
		}
	}

	return nil
}

func (ss *session) Start() error {
	for i := 0; i < len(ss.temporalWorkers); i++ {
		err := ss.temporalWorkers[i].Start()
		if err != nil {
			return err
		}
	}
	return nil
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the ss.
func (ss *session) Destroy(ctx context.Context) {
	for i := 0; i < len(ss.temporalWorkers); i++ {
		ss.temporalWorkers[i].Stop()
	}

	// TODO add ctx.Done in RR for timeouts
	ss.workerPool.Destroy(ctx)
}

// todo: improve this method handling, introduce proper marshalling and faster decoding.
// todo: wrap execution into command.
func (ss *session) handleActivity(ctx context.Context, data rrt.RRPayload) (rrt.RRPayload, error) {
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

	res, err := ss.workerPool.Exec(payload)
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
