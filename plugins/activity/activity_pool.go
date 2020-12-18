package activity

import (
	"context"
	"sync/atomic"

	jsoniter "github.com/json-iterator/go"

	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/interfaces/events"
	"github.com/spiral/roadrunner/v2/interfaces/pool"
	"github.com/spiral/roadrunner/v2/interfaces/server"
	rrWorker "github.com/spiral/roadrunner/v2/interfaces/worker"
	poolImpl "github.com/spiral/roadrunner/v2/pkg/pool"
	rrt "github.com/temporalio/roadrunner-temporal"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/worker"
)

type (
	activityPool interface {
		Start(ctx context.Context, temporal temporal.Temporal) error
		Destroy(ctx context.Context) error
		Workers() []rrWorker.BaseProcess
		ActivityNames() []string
	}

	activityPoolImpl struct {
		dc         converter.DataConverter
		seqID      uint64
		events     events.Handler
		activities []string
		wp         pool.Pool
		tWorkers   []worker.Worker
	}
)

// newActivityPool
func newActivityPool(listener events.EventListener, poolConfig poolImpl.Config, server server.Server) (activityPool, error) {
	wp, err := server.NewWorkerPool(
		context.Background(),
		poolConfig,
		map[string]string{"RR_MODE": RRMode},
	)

	if err != nil {
		return nil, err
	}

	wp.AddListener(listener)

	return &activityPoolImpl{wp: wp}, nil
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the pool.
func (pool *activityPoolImpl) Start(ctx context.Context, temporal temporal.Temporal) error {
	pool.dc = temporal.GetDataConverter()

	err := pool.initWorkers(ctx, temporal)
	if err != nil {
		return err
	}

	for i := 0; i < len(pool.tWorkers); i++ {
		err := pool.tWorkers[i].Start()
		if err != nil {
			return err
		}
	}

	return nil
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the pool.
func (pool *activityPoolImpl) Destroy(ctx context.Context) error {
	for i := 0; i < len(pool.tWorkers); i++ {
		pool.tWorkers[i].Stop()
	}

	pool.wp.Destroy(ctx)
	return nil
}

func (pool *activityPoolImpl) Workers() []rrWorker.BaseProcess {
	return pool.wp.Workers()
}

func (pool *activityPoolImpl) ActivityNames() []string {
	return pool.activities
}

// initWorkers request workers workflows from underlying PHP and configures temporal workers linked to the pool.
func (pool *activityPoolImpl) initWorkers(ctx context.Context, temporal temporal.Temporal) error {
	workerInfo, err := rrt.GetWorkerInfo(pool.wp)
	if err != nil {
		return err
	}

	pool.activities = make([]string, 0)
	pool.tWorkers = make([]worker.Worker, 0)

	for _, info := range workerInfo {
		w, err := temporal.CreateWorker(info.TaskQueue, info.Options)
		if err != nil {
			return errors.E(errors.Op("createTemporalWorker"), err, pool.Destroy(ctx))
		}

		pool.tWorkers = append(pool.tWorkers, w)
		for _, activityInfo := range info.Activities {
			w.RegisterActivityWithOptions(pool.executeActivity, activity.RegisterOptions{
				Name:                          activityInfo.Name,
				DisableAlreadyRegisteredCheck: false,
			})

			pool.activities = append(pool.activities, activityInfo.Name)
		}
	}

	return nil
}

// executes activity with underlying worker.
func (pool *activityPoolImpl) executeActivity(ctx context.Context, input rrt.RRPayload) (rrt.RRPayload, error) {
	var (
		// todo: activity.getHeartBeatDetails

		err  error
		info = activity.GetInfo(ctx)
		msg  = rrt.Message{
			Command: InvokeActivityCommand,
			ID:      atomic.AddUint64(&pool.seqID, 1),
		}
		cmd = InvokeActivity{
			Name: info.ActivityType.Name,
			Info: info,
		}
	)

	/*
		TODO: next iteration should avoid processing payloads anyhow, the processing should be done on PHP end.
	*/

	// todo: optimize
	for _, value := range input.Data {
		vData, err := jsoniter.Marshal(value)
		if err != nil {
			return rrt.RRPayload{}, err
		}

		cmd.Args = append(cmd.Args, vData)
	}

	msg.Params, err = jsoniter.Marshal(cmd)
	if err != nil {
		return rrt.RRPayload{}, err
	}

	result, err := rrt.Execute(pool.wp, rrt.Context{TaskQueue: info.TaskQueue}, msg)
	if err != nil {
		return rrt.RRPayload{}, err
	}

	if len(result) != 1 {
		return rrt.RRPayload{}, errors.E(errors.Op("executeActivity"), "invalid activity worker response")
	}

	if result[0].Error != nil {
		if result[0].Error.Message == "doNotCompleteOnReturn" {
			return rrt.RRPayload{}, activity.ErrResultPending
		}

		return rrt.RRPayload{}, errors.E(result[0].Error.Message)
	}

	// todo: optimize
	out := rrt.RRPayload{}
	for _, raw := range result[0].Result {
		var value interface{}
		err := jsoniter.Unmarshal(raw, &value)
		if err != nil {
			return rrt.RRPayload{}, err
		}

		out.Data = append(out.Data, value)
	}

	return out, nil
}
