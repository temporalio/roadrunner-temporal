package activity

import (
	"context"
	"github.com/spiral/endure/errors"
	"github.com/spiral/roadrunner/v2"
	"github.com/spiral/roadrunner/v2/plugins/app"
	"log"

	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
)

const RRMode = "temporal/activities"

type Server struct {
	temporal temporal.Temporal
	app      app.WorkerFactory
	ss       *session
}

// logger dep also
func (srv *Server) Init(temporal temporal.Temporal, app app.WorkerFactory) error {
	if temporal.GetConfig().Activities == nil {
		// no need to serve activities
		return errors.E(errors.Disabled)
	}

	srv.temporal = temporal
	srv.app = app

	return nil
}

func (srv *Server) Serve() chan error {
	errCh := make(chan error, 1)

	pool, err := srv.initPool()
	if err != nil {
		errCh <- errors.E(errors.Op("init ss"), err)
		return errCh
	}

	// set the ss after all initialization complete
	srv.ss = pool

	return errCh
}

func (srv *Server) Stop() error {
	if srv.ss != nil {
		srv.ss.Destroy(context.Background())
	}

	return nil
}

// non blocking function
func (srv *Server) initPool() (*session, error) {
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

func (srv *Server) createPool(ctx context.Context) (*session, error) {
	rrPool, err := srv.app.NewWorkerPool(
		context.Background(),
		*srv.temporal.GetConfig().Activities,
		map[string]string{"RR_MODE": RRMode},
	)

	if err != nil {
		return nil, err
	}

	// todo: observe ss events and restart it
	rrPool.AddListener(func(event interface{}) {
		if pe, ok := event.(roadrunner.PoolEvent); ok {
			if pe.Event == roadrunner.EventPoolError {
				srv.recreatePool()
			}
		}
	})

	pool := newSession(rrPool)
	err = pool.InitPool(ctx, srv.temporal)
	if err != nil {
		return nil, err
	}

	return pool, nil
}

func (srv *Server) recreatePool() {
	// todo: implement
	log.Print("ss is dead")
}
