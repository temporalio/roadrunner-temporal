package temporal

import (
	"context"
	"github.com/spiral/roadrunner/v2/plugins/factory"
)

type ActivityRouter struct {
	temporal *Provider
	wFactory factory.WorkerFactory

	// currently active worker pool (can be replaced at runtime)
	pool *ActivityPool
}

// logger dep also
func (a *ActivityRouter) Init(temporal *Provider, wFactory factory.WorkerFactory) error {
	a.temporal = temporal
	a.wFactory = wFactory
	return nil
}

func (a *ActivityRouter) Serve() chan error {
	errCh := make(chan error)
	if a.temporal.config.Activities != nil {
		go a.initPool(errCh)
	}

	return errCh
}

func (a *ActivityRouter) initPool(errCh chan error) {
	pool, err := a.createPool()
	if err != nil {
		errCh <- err
		return
	}

	a.pool = pool
	go a.pool.Serve(errCh)
}

func (a *ActivityRouter) Stop() error {
	if a.pool != nil {
		a.pool.Destroy(context.Background())
	}

	return nil
}

func (a *ActivityRouter) createPool() (pool *ActivityPool, err error) {
	pool = &ActivityPool{}

	pool.workerPool, err = a.wFactory.NewWorkerPool(
		context.Background(),
		a.temporal.config.Activities,
		map[string]string{"RR_MODE": "temporal/activities"},
	)

	if err != nil {
		return nil, err
	}

	if err := pool.InitTemporal(a.temporal); err != nil {
		return nil, err
	}

	return pool, nil
}
