package workflow

import (
	"context"
	"github.com/spiral/endure/errors"
	"github.com/spiral/roadrunner/v2/plugins/app"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"go.uber.org/zap"
)

const (
	// ServiceName defines public service name.
	ServiceName = "workflows"

	// RRMode sets as RR_MODE env variable to let worker know about the mode to run.
	RRMode = "temporal/workflow"
)

// Service manages workflows and workers.
type Service struct {
	temporal temporal.Temporal
	app      app.WorkerFactory
	log      *zap.Logger
	pool     *workflowPool
}

// logger dep also
func (svc *Service) Init(temporal temporal.Temporal, app app.WorkerFactory) error {
	svc.temporal = temporal
	svc.app = app
	return nil
}

// Serve starts workflow service.
func (svc *Service) Serve() chan error {
	errCh := make(chan error, 1)

	pool, err := NewWorkflowPool(context.Background(), svc.app)
	if err != nil {
		errCh <- errors.E(errors.Op("NewWorkflowPool"), err)
		return errCh
	}

	err = pool.Start(context.Background(), svc.temporal)
	if err != nil {
		errCh <- errors.E(errors.Op("startWorkflowPool"), err)
		return errCh
	}

	svc.pool = pool

	return errCh
}

// todo: add hot reload
// todo: add event listener

// Stop workflow service.
func (svc *Service) Stop() error {
	if svc.pool != nil {
		svc.pool.Destroy(context.Background())
	}

	return nil
}

// Name of the service.
func (svc *Service) Name() string {
	return ServiceName
}
