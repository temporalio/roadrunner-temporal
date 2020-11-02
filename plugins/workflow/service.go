package workflow

import (
	"context"

	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/plugins/app"
	"github.com/spiral/roadrunner/v2/util"
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
	events   *util.EventHandler
	app      app.WorkerFactory
	log      *zap.Logger
	pool     *workflowPool
}

// logger dep also
func (svc *Service) Init(temporal temporal.Temporal, app app.WorkerFactory, log *zap.Logger) error {
	svc.temporal = temporal
	svc.app = app
	svc.log = log
	return nil
}

// Serve starts workflow service.
func (svc *Service) Serve() chan error {
	errCh := make(chan error, 1)

	pool, err := NewWorkflowPool(context.Background(), svc.app)
	if err != nil {
		errCh <- errors.E(errors.Op("newWorkflowPool"), err)
		return errCh
	}

	// todo: proxy events

	err = pool.Start(context.Background(), svc.temporal)
	if err != nil {
		errCh <- errors.E(errors.Op("startWorkflowPool"), err)
		return errCh
	}

	svc.pool = pool

	var workflows []string
	for name, _ := range pool.workflows {
		workflows = append(workflows, name)
	}

	svc.log.Debug("Started workflow processing", zap.Any("workflows", workflows))

	return errCh
}

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

// Reset resets underlying workflow pool with new copy.
func (svc *Service) Reset() error {
	// todo: implement
	return nil
}

// AddListener adds event listeners to the service.
func (svc *Service) AddListener(listener util.EventListener) {
	svc.events.AddListener(listener)
}

// todo: workers method
