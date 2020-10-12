package factory

import (
	"context"
	"github.com/temporalio/roadrunner-temporal/events"

	"github.com/temporalio/roadrunner-temporal/roadrunner"
)

type WorkerFactory interface {
	NewWorker(ctx context.Context, env Env) (roadrunner.WorkerBase, error)
	NewWorkerPool(ctx context.Context, opt *roadrunner.Config, env Env) (roadrunner.Pool, error)
}

type WFactory struct {
	spw Spawner
	eb  *events.EventBroadcaster
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

	p, err := roadrunner.NewPool(ctx, cmd, factory, opt)
	if err != nil {
		return nil, err
	}

	go func() {
		for e := range p.Events() {
			wf.eb.Push(e)
		}
	}()

	return p, nil
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
	wf.eb = events.NewEventBroadcaster()
	//wf.config = config
	return nil
}

func (wf *WFactory) AddListener(l events.EventListener) {
	wf.eb.AddListener(l)
}

// TODO make serve stop optional
func (wf *WFactory) Serve() chan error {
	c := make(chan error)
	return c
}

func (wf *WFactory) Stop() error {
	return nil
}
