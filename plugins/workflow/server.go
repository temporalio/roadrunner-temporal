package workflow

import (
	"context"
	"github.com/spiral/endure/errors"
	"github.com/spiral/roadrunner/v2"
	"github.com/spiral/roadrunner/v2/plugins/factory"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"log"
)

const RRMode = "temporal/workflows"

type Server struct {
	temporal temporal.Temporal
	app      factory.AppFactory
	ss       *session
}

// logger dep also
func (srv *Server) Init(temporal temporal.Temporal, app factory.AppFactory) error {
	srv.temporal = temporal
	srv.app = app
	return nil
}

func (srv *Server) Serve() chan error {
	errCh := make(chan error, 1)

	session, err := srv.initSession()
	if err != nil {
		errCh <- errors.E(errors.Op("init ss"), err)
		return errCh
	}

	// start the ss
	srv.ss = session

	return errCh
}

func (srv *Server) Stop() error {
	if srv.ss != nil {
		srv.ss.Destroy(context.Background())
	}

	return nil
}

func (srv *Server) initSession() (*session, error) {
	session, err := srv.createSession(context.Background())
	if err != nil {
		return nil, err
	}

	err = session.Start()
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (srv *Server) createSession(ctx context.Context) (*session, error) {
	worker, err := srv.app.NewWorker(
		context.Background(),
		map[string]string{"RR_MODE": RRMode},
	)
	if err != nil {
		return nil, err
	}

	worker.AddListener(func(event interface{}) {
		if event.(roadrunner.WorkerEvent).Event == roadrunner.EventWorkerError {
			// todo: recreate ss
		}
	})

	go func() {
		err := worker.Wait(ctx)

		log.Print(err)
		// todo: recreate ss
	}()

	session, err := newSession(worker)
	if err != nil {
		return nil, err
	}

	err = session.InitSession(ctx, srv.temporal)
	if err != nil {
		return nil, err
	}

	return session, nil
}
