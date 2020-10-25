package workflow

import (
	"context"
	"github.com/spiral/roadrunner/v2/plugins/factory"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
)

const RRMode = "temporal/workflows"

type Server struct {
	temporal temporal.Temporal
	wFactory factory.WorkerFactory

	// currently active workflowProxy execution session
	session *session
}

// logger dep also
func (srv *Server) Init(temporal temporal.Temporal, wFactory factory.WorkerFactory) error {
	srv.temporal = temporal
	srv.wFactory = wFactory
	return nil
}

func (srv *Server) Serve() chan error {
	errCh := make(chan error, 1)

	session, err := srv.initSession()
	if err != nil {
		errCh <- err
		return errCh
	}

	// start the session
	srv.session = session

	return errCh
}

func (srv *Server) Stop() error {
	if srv.session != nil {
		srv.session.Destroy(context.Background())
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
	worker, err := srv.wFactory.NewWorker(
		context.Background(),
		map[string]string{"RR_MODE": RRMode},
	)

	if err != nil {
		return nil, err
	}

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
