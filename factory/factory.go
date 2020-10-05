package factory

import (
	"context"

	"github.com/temporalio/roadrunner-temporal/config"
	"github.com/temporalio/roadrunner-temporal/roadrunner"
)

type WorkerFactory interface {
	NewWorker(ctx context.Context, env Env) (roadrunner.WorkerBase, error)
	NewWorkerPool(ctx context.Context, opt roadrunner.Config, env Env) (roadrunner.Pool, error)
}

type WFactory struct {
	spw    Spawner
	config config.Provider
}

func (wf *WFactory) NewWorkerPool(ctx context.Context, opt roadrunner.Config, env Env) (roadrunner.Pool, error) {
	cmd, err := wf.spw.NewCmd(env)
	if err != nil {
		return nil, err
	}
	factory, err := wf.spw.NewFactory(env)
	if err != nil {
		return nil, err
	}

	return roadrunner.NewPool(cmd, factory, opt)
}

func (wf *WFactory) NewWorker(ctx context.Context, env Env) (roadrunner.WorkerBase, error) {
	return nil, nil
}

func (wf *WFactory) Init(app Spawner, config config.Provider) error {
	wf.spw = app
	wf.config = config
	return nil
}

func (wf *WFactory) Serve() chan error {
	c := make(chan error)
	pool, err := wf.NewWorkerPool(context.Background(), roadrunner.Config{
		NumWorkers:      0,
		MaxJobs:         0,
		AllocateTimeout: 0,
		DestroyTimeout:  0,
		TTL:             0,
		IdleTTL:         0,
		ExecTTL:         0,
		MaxPoolMemory:   0,
		MaxWorkerMemory: 0,
	}, Env{})
	if err != nil {
		c <- err
	}
	_ = pool
	return c
}

func (wf *WFactory) Stop() error {
	return nil
}
