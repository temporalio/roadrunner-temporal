package temporal

import (
	"context"
	"github.com/spiral/roadrunner/v2/plugins/factory"
)

type ActivityFactory struct {
	temporal *Provider
	wFactory factory.WorkerFactory

	// currently active worker pool (can be replaced at runtime)
	pool *ActivityPool
}

// logger dep also
func (a *ActivityFactory) Init(temporal *Provider, wFactory factory.WorkerFactory) error {
	a.temporal = temporal
	a.wFactory = wFactory
	return nil
}

func (a *ActivityFactory) Serve() chan error {
	errCh := make(chan error)
	if a.temporal.config.Activities != nil {
		go a.initPool(errCh)
	}

	return errCh
}

func (a *ActivityFactory) initPool(errCh chan error) {
	pool, err := a.createPool(context.Background())
	if err != nil {
		errCh <- err
		return
	}

	a.pool = pool
	go a.pool.Start(errCh)
}

func (a *ActivityFactory) Stop() error {
	if a.pool != nil {
		a.pool.Destroy(context.Background())
	}

	return nil
}

func (a *ActivityFactory) createPool(ctx context.Context) (pool *ActivityPool, err error) {
	pool = &ActivityPool{}

	pool.workerPool, err = a.wFactory.NewWorkerPool(
		context.Background(),
		a.temporal.config.Activities,
		map[string]string{"RR_MODE": "temporal/activities"},
	)

	if err != nil {
		return nil, err
	}

	if err := pool.InitTemporal(ctx, a.temporal); err != nil {
		return nil, err
	}

	return pool, nil
}
