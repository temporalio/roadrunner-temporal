package activity

import (
	"context"

	"github.com/spiral/roadrunner/v2/plugins/factory"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
)

const RRMode = "temporal/activities"

type Server struct {
	temporal temporal.Temporal
	wFactory factory.WorkerFactory

	// currently active worker pool (can be replaced at runtime)
	pool *activityPool
}

// logger dep also
func (srv *Server) Init(temporal temporal.Temporal, wFactory factory.WorkerFactory) error {
	srv.temporal = temporal
	srv.wFactory = wFactory
	return nil
}

func (srv *Server) Serve() chan error {
	errCh := make(chan error, 1)
	if srv.temporal.GetConfig().Activities != nil {
		pool, err := srv.initPool()
		if err != nil {
			errCh <- err
			return errCh
		}

		// set the pool after all initialization complete
		srv.pool = pool
	}

	return errCh
}

// non blocking function
func (srv *Server) initPool() (*activityPool, error) {
	pool, err := srv.createPool(context.Background())
	if err != nil {
		return nil, err
	}

	err = pool.Start()
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func (srv *Server) Stop() error {
	if srv.pool != nil {
		srv.pool.Destroy(context.Background())
	}

	return nil
}

func (srv *Server) createPool(ctx context.Context) (*activityPool, error) {
	rrPool, err := srv.wFactory.NewWorkerPool(
		context.Background(),
		srv.temporal.GetConfig().Activities,
		map[string]string{"RR_MODE": RRMode},
	)

	if err != nil {
		return nil, err
	}

	pool := newActivityPool(rrPool)
	err = pool.InitPool(ctx, srv.temporal)
	if err != nil {
		return nil, err
	}

	return pool, nil
}
