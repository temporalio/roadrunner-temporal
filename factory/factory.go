package factory

import (
	"context"

	"github.com/temporalio/roadrunner-temporal/roadrunner"
)

type WorkerFactory interface {
	NewWorker(ctx context.Context, env Env) (roadrunner.WorkerBase, error)
	NewWorkerPool(ctx context.Context, opt *roadrunner.Config, env Env) (roadrunner.Pool, error)
}

type WFactory struct {
	spw    Spawner
	//config config.Provider
}

func (wf *WFactory) NewWorkerPool(ctx context.Context, opt *roadrunner.Config, env Env) (roadrunner.Pool, error) {
	cmd, err := wf.spw.NewCmd(env)
	if err != nil {
		return nil, err
	}
	factory, err := wf.spw.NewFactory(env)
	if err != nil {
		return nil, err
	}

	return roadrunner.NewPool(ctx, cmd, factory, opt)
}

func (wf *WFactory) NewWorker(ctx context.Context, env Env) (roadrunner.WorkerBase, error) {
	cmd, err := wf.spw.NewCmd(env)
	if err != nil {
		return nil, err
	}

	wb, err := roadrunner.InitBaseWorker(cmd())
	if err != nil {
		return nil, err
	}

	return wb, nil
}

func (wf *WFactory) Init(app Spawner) error {
	wf.spw = app
	//wf.config = config
	return nil
}

// TODO make serve stop optional
func (wf *WFactory) Serve() chan error {
	c := make(chan error)
	return c
}

func (wf *WFactory) Stop() error {
	return nil
}