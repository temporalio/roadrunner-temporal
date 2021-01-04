package activity

import (
	"context"

	"github.com/cenkalti/backoff/v4"
	"github.com/spiral/roadrunner/v2/interfaces/events"
	"github.com/spiral/roadrunner/v2/interfaces/worker"
	eventsImpl "github.com/spiral/roadrunner/v2/pkg/events"

	"sync"
	"sync/atomic"

	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/plugins/logger"
	"github.com/spiral/roadrunner/v2/plugins/server"
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
	events   events.Handler
	server   server.Server
	log      logger.Logger
	mu       sync.Mutex
	reset    chan struct{}
	pool     activityPool
	closing  int64
}

// Init configures activity service.
func (svc *Plugin) Init(temporal temporal.Temporal, server server.Server, log logger.Logger) error {
	if temporal.GetConfig().Activities == nil {
		// no need to serve activities
		return errors.E(errors.Disabled)
	}

	svc.temporal = temporal
	svc.server = server
	svc.events = eventsImpl.NewEventsHandler()
	svc.log = log
	svc.reset = make(chan struct{})

	return nil
}

// Serve activities with underlying workers.
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

// Workers returns pool workers.
func (svc *Plugin) Workers() []worker.BaseProcess {
	return svc.getPool().Workers()
}

// ActivityNames returns list of all available activities.
func (svc *Plugin) ActivityNames() []string {
	return svc.pool.ActivityNames()
}

// Reset resets underlying workflow pool with new copy.
func (svc *Plugin) Reset() error {
	svc.reset <- struct{}{}

	return nil
}

// AddListener adds event listeners to the service.
func (svc *Plugin) AddListener(listener events.Listener) {
	svc.events.AddListener(listener)
}

// AddListener adds event listeners to the service.
func (svc *Plugin) poolListener(event interface{}) {
	if ev, ok := event.(events.PoolEvent); ok {
		if ev.Event == events.EventPoolError {
			svc.log.Error("Activity pool error", "error", ev.Payload.(error))
			svc.reset <- struct{}{}
		}
	}

	svc.events.Push(event)
}

// AddListener adds event listeners to the service.
func (svc *Plugin) startPool() (activityPool, error) {
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

	pool, err := svc.startPool()
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
			errors.E(errors.Op("destroyActivityPool"), errD),
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
