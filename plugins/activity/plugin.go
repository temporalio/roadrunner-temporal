package activity

import (
	"context"
	"github.com/cenkalti/backoff/v4"
	"sync"

	"github.com/spiral/roadrunner/v2"

	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/interfaces/log"
	"github.com/spiral/roadrunner/v2/interfaces/server"
	"github.com/spiral/roadrunner/v2/util"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
)

const (
	// PluginName defines public service name.
	PluginName = "activities"

	// RRMode sets as RR_MODE env variable to let worker know about the mode to run.
	RRMode = "temporal/activity"
)

// Plugin to manage activity execution.
type Plugin struct {
	temporal temporal.Temporal
	events   util.EventsHandler
	server   server.Server
	log      log.Logger
	mu       sync.Mutex
	reset    chan struct{}
	pool     activityPool
}

// Init configures activity service.
func (svc *Plugin) Init(temporal temporal.Temporal, server server.Server, log log.Logger) error {
	if temporal.GetConfig().Activities == nil {
		// no need to serve activities
		return errors.E(errors.Disabled)
	}

	svc.temporal = temporal
	svc.server = server
	svc.log = log

	return nil
}

// Serve activities with underlying workers.
func (svc *Plugin) Serve() chan error {
	errCh := make(chan error, 1)

	pool, err := svc.initPool()
	if err != nil {
		errCh <- errors.E("initPool", err)
		return errCh
	}

	svc.pool = pool

	go func() {
		for {
			select {
			case <-svc.reset:
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

func (svc *Plugin) Stop() error {
	pool := svc.getPool()
	if pool != nil {
		svc.pool = nil
		return pool.Destroy(context.Background())
	}

	close(svc.reset)

	return nil
}

// Name of the service.
func (svc *Plugin) Name() string {
	return PluginName
}

// Workers returns pool workers.
func (svc *Plugin) Workers() []roadrunner.WorkerBase {
	return svc.getPool().Workers()
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
	case roadrunner.PoolEvent:
		if p.Event == roadrunner.EventPoolError {
			svc.log.Error("Activity pool error", "error", p.Payload.(error))
			svc.reset <- struct{}{}
		}
	}

	svc.events.Push(event)
}

// AddListener adds event listeners to the service.
func (svc *Plugin) initPool() (activityPool, error) {
	pool, err := newActivityPool(svc.poolListener, *svc.temporal.GetConfig().Activities, svc.server)

	if err != nil {
		return nil, errors.E(errors.Op("newActivityPool"), err)
	}

	err = pool.Start(context.Background(), svc.temporal)
	if err != nil {
		return nil, errors.E(errors.Op("startActivityPool"), err)
	}

	svc.log.Debug("Started activity processing", "activities", pool.ActivityNames())

	return pool, nil
}

func (svc *Plugin) replacePool() error {
	svc.log.Debug("Replace activity pool")

	pool, err := svc.initPool()
	if err != nil {
		return errors.E(errors.Op("newActivityPool"), err)
	}

	var previous activityPool

	svc.mu.Lock()
	previous, svc.pool = svc.pool, pool
	svc.mu.Unlock()

	errD := previous.Destroy(context.Background())
	if errD != nil {
		svc.log.Error(
			"Unable to destroy expired activity pool",
			"error",
			errors.E(errors.Op("destroyActivityPool"), err),
		)
	}

	return nil
}

// getPool returns currently pool.
func (svc *Plugin) getPool() activityPool {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	return svc.pool
}
