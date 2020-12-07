package workflow

import (
	"context"
	"github.com/cenkalti/backoff/v4"
	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2"
	"github.com/spiral/roadrunner/v2/interfaces/log"
	"github.com/spiral/roadrunner/v2/interfaces/server"
	"github.com/spiral/roadrunner/v2/util"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"sync"
	"sync/atomic"
)

const (
	// PluginName defines public service name.
	PluginName = "workflows"

	// RRMode sets as RR_MODE env variable to let worker know about the mode to run.
	RRMode = "temporal/workflow"
)

// Plugin manages workflows and workers.
type Plugin struct {
	temporal temporal.Temporal
	events   util.EventsHandler
	server   server.Server
	log      log.Logger
	mu       sync.Mutex
	reset    chan struct{}
	pool     workflowPool
	closing  int64
}

// logger dep also
func (svc *Plugin) Init(temporal temporal.Temporal, server server.Server, log log.Logger) error {
	svc.temporal = temporal
	svc.server = server
	svc.events = util.NewEventsHandler()
	svc.log = log
	svc.reset = make(chan struct{})

	return nil
}

// Serve starts workflow service.
func (svc *Plugin) Serve() chan error {
	errCh := make(chan error, 1)

	pool, err := svc.startPool()
	if err != nil {
		errCh <- errors.E("startPool", err)
		return errCh
	}

	svc.pool = pool

	go func() {
		for {
			select {
			case <-svc.reset:
				if atomic.LoadInt64(&svc.closing) == 1 {
					return
				}

				err := svc.replacePool()
				if err == nil {
					continue
				}

				bkoff := backoff.NewExponentialBackOff()

				err = backoff.Retry(svc.replacePool, bkoff)
				if err != nil {
					errCh <- errors.E("deadPool", err)
				}
			}
		}
	}()

	return errCh
}

// Stop workflow service.
func (svc *Plugin) Stop() error {
	atomic.StoreInt64(&svc.closing, 1)

	pool := svc.getPool()
	if pool != nil {
		svc.pool = nil
		return pool.Destroy(context.Background())
	}

	return nil
}

// Name of the service.
func (svc *Plugin) Name() string {
	return PluginName
}

// Name of the service.
func (svc *Plugin) Workers() []roadrunner.WorkerBase {
	return svc.pool.Workers()
}

// Reset resets underlying workflow pool with new copy.
func (svc *Plugin) Reset() error {
	svc.reset <- struct{}{}

	return nil
}

// AddListener adds event listeners to the service.
func (svc *Plugin) AddListener(listener util.EventListener) {
	svc.events.AddListener(listener)
}

// AddListener adds event listeners to the service.
func (svc *Plugin) poolListener(event interface{}) {
	switch p := event.(type) {
	case PoolEvent:
		if p.Event == EventWorkerExit {
			svc.log.Error("Workflow pool error", "error", p.Caused)
			svc.reset <- struct{}{}
		}
	}

	svc.events.Push(event)
}

// AddListener adds event listeners to the service.
func (svc *Plugin) startPool() (workflowPool, error) {
	pool, err := newWorkflowPool(svc.poolListener, svc.server)
	if err != nil {
		return nil, errors.E(errors.Op("initWorkflowPool"), err)
	}

	err = pool.Start(context.Background(), svc.temporal)
	if err != nil {
		return nil, errors.E(errors.Op("startWorkflowPool"), err)
	}

	svc.log.Debug("Started workflow processing", "workflows", pool.WorkflowNames())

	return pool, nil
}

func (svc *Plugin) replacePool() error {
	svc.log.Debug("Replace workflow pool")

	pool, err := svc.startPool()
	if err != nil {
		return errors.E(errors.Op("newWorkflowPool"), err)
	}

	var previous workflowPool

	svc.mu.Lock()
	previous, svc.pool = svc.pool, pool
	svc.mu.Unlock()

	errD := previous.Destroy(context.Background())
	if errD != nil {
		svc.log.Error(
			"Unable to destroy expired workflow pool",
			"error",
			errors.E(errors.Op("destroyWorkflowPool"), errD),
		)
	}

	return nil
}

// getPool returns currently pool.
func (svc *Plugin) getPool() workflowPool {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	return svc.pool
}
