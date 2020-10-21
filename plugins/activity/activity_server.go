package activity_server

import (
	"context"

	"github.com/spiral/roadrunner/v2/plugins/factory"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
)

type ActivityServer struct {
	temporal temporal.Temporal
	wFactory factory.WorkerFactory

	// currently active worker pool (can be replaced at runtime)
	pool ActivityPool
}

// logger dep also
func (a *ActivityServer) Init(temporal temporal.Temporal, wFactory factory.WorkerFactory) error {
	a.temporal = temporal
	a.wFactory = wFactory
	return nil
}

func (a *ActivityServer) Serve() chan error {
	errCh := make(chan error, 1)
	if a.temporal.GetConfig().Activities != nil {
		pool, err := a.initPool()
		if err != nil {
			errCh <- err
			return errCh
		}

		// set the pool after all initialization complete
		a.pool = pool
	}

	return errCh
}

// non blocking function
func (a *ActivityServer) initPool() (ActivityPool, error) {
	pool, err := a.createPool(context.Background())
	if err != nil {
		return nil, err
	}

	err = pool.Start()
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func (a *ActivityServer) Stop() error {
	if a.pool != nil {
		a.pool.Destroy(context.Background())
	}

	return nil
}

func (a *ActivityServer) createPool(ctx context.Context) (ActivityPool, error) {
	rrPool, err := a.wFactory.NewWorkerPool(
		context.Background(),
		a.temporal.GetConfig().Activities,
		map[string]string{"RR_MODE": "temporal/activities"},
	)
	if err != nil {
		return nil, err
	}

	pool := NewActivityPool(rrPool)
	err = pool.InitTemporal(ctx, a.temporal)
	if err != nil {
		return nil, err
	}

	return pool, nil
}
