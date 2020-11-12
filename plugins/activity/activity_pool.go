package activity

import (
	"context"
	"sync/atomic"

	jsoniter "github.com/json-iterator/go"

	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2"
	"github.com/spiral/roadrunner/v2/plugins/app"
	"github.com/spiral/roadrunner/v2/util"
	rrt "github.com/temporalio/roadrunner-temporal"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/worker"
)

// activityPool manages set of RR and Temporal activity workers and their cancellation contexts.
type activityPool struct {
	dc         converter.DataConverter
	seqID      uint64
	events     *util.EventHandler
	activities []string
	wp         roadrunner.Pool
	tWorkers   []worker.Worker
}

// NewActivityPool
func NewActivityPool(ctx context.Context, poolConfig roadrunner.Config, factory app.WorkerFactory) (*activityPool, error) {
	wp, err := factory.NewWorkerPool(
		context.Background(),
		poolConfig,
		map[string]string{"RR_MODE": RRMode},
	)

	if err != nil {
		return nil, err
	}

	wp.AddListener(func(event interface{}) {
		if _, ok := event.(roadrunner.PoolEvent); ok {
			return
		}
	})

	return &activityPool{wp: wp}, nil
}

// AddListener adds event listeners to the workflow pool.
func (pool *activityPool) AddListener(listener util.EventListener) {
	pool.events.AddListener(listener)
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the pool.
func (pool *activityPool) Start(ctx context.Context, temporal temporal.Temporal) error {
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
func (pool *activityPool) Destroy(ctx context.Context) {
	for i := 0; i < len(pool.tWorkers); i++ {
		pool.tWorkers[i].Stop()
	}

	// TODO add ctx.Done in RR for timeouts
	pool.wp.Destroy(ctx)
}

// initWorkers request workers workflows from underlying PHP and configures temporal workers linked to the pool.
func (pool *activityPool) initWorkers(ctx context.Context, temporal temporal.Temporal) error {
	workerInfo, err := rrt.GetWorkerInfo(pool.wp)
	if err != nil {
		return err
	}

	pool.activities = make([]string, 0)
	pool.tWorkers = make([]worker.Worker, 0)

	for _, info := range workerInfo {
		w, err := temporal.CreateWorker(info.TaskQueue, info.Options.TemporalOptions())
		if err != nil {
			pool.Destroy(ctx)
			return errors.E(errors.Op("createTemporalWorker"), err)
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
func (pool *activityPool) executeActivity(ctx context.Context, input rrt.RRPayload) (rrt.RRPayload, error) {
	var (
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

	// todo: handle error and async commands
	return out, nil
}
