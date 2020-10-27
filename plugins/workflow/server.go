package workflow

import (
	"context"
	"github.com/fatih/color"
	"github.com/spiral/endure/errors"
	"github.com/spiral/roadrunner/v2"
	"github.com/spiral/roadrunner/v2/plugins/app"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"log"
)

const RRMode = "temporal/workflow"

type Server struct {
	temporal temporal.Temporal
	app      app.WorkerFactory
	ss       *workflowPool
}

// logger dep also
func (srv *Server) Init(temporal temporal.Temporal, app app.WorkerFactory) error {
	srv.temporal = temporal
	srv.app = app
	return nil
}

func (srv *Server) Serve() chan error {
	errCh := make(chan error, 1)

	session, err := srv.initSession()
	if err != nil {
		errCh <- errors.E(errors.Op("workflowServer"), err)
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

func (srv *Server) initSession() (*workflowPool, error) {
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

func (srv *Server) createSession(ctx context.Context) (*workflowPool, error) {
	worker, err := srv.app.NewWorker(
		context.Background(),
		map[string]string{"RR_MODE": RRMode},
	)
	if err != nil {
		return nil, err
	}

	worker.AddListener(func(event interface{}) {
		if event.(roadrunner.WorkerEvent).Event == roadrunner.EventWorkerLog {
			// todo: recreate ss
			log.Print(color.RedString(string(event.(roadrunner.WorkerEvent).Payload.([]byte))))
		}
	})

	go func() {
		err := worker.Wait(ctx)

		log.Print(err)
		// todo: recreate ss
	}()

	session, err := newWorkflowPool(worker)
	if err != nil {
		return nil, err
	}

	err = session.InitSession(ctx, srv.temporal)
	if err != nil {
		return nil, err
	}

	return session, nil
}
