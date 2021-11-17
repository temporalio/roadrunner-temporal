package activity

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/spiral/roadrunner-plugins/v2/config"
	"github.com/spiral/roadrunner/v2/events"
	"github.com/spiral/roadrunner/v2/state/process"
	rrWorker "github.com/spiral/roadrunner/v2/worker"
	roadrunner_temporal "github.com/temporalio/roadrunner-temporal"

	"github.com/spiral/errors"
	"github.com/spiral/roadrunner-plugins/v2/logger"
	"github.com/spiral/roadrunner-plugins/v2/server"
	"github.com/temporalio/roadrunner-temporal/client"
)

const (
	// PluginName defines public service name.
	PluginName = "activities"
)

// Plugin to manage activity execution.
type Plugin struct {
	temporal client.Temporal

	events events.EventBus
	id     string

	server server.Server
	log    logger.Logger
	mu     sync.Mutex
	reset  chan struct{}
	stopCh chan struct{}
	pool   activityPool
	// graceful timeout for the worker
	graceTimeout time.Duration
	closing      int64
}

// Init configures activity service.
func (p *Plugin) Init(temporal client.Temporal, server server.Server, log logger.Logger, cfg config.Configurer) error {
	const op = errors.Op("activity_plugin_init")
	if !cfg.Has(roadrunner_temporal.RootPluginName) {
		return errors.E(op, errors.Disabled)
	}

	if temporal.GetConfig().Activities == nil {
		// no need to serve activities
		return errors.E(op, errors.Disabled)
	}

	p.temporal = temporal
	p.server = server
	p.events, p.id = events.Bus()
	p.log = log
	p.reset = make(chan struct{})
	p.stopCh = make(chan struct{})

	// it can't be 0 (except set by user), because it would be set by the rr-binary (cli)
	p.graceTimeout = cfg.GetCommonConfig().GracefulTimeout

	return nil
}

// Serve activities with underlying workers.
func (p *Plugin) Serve() chan error {
	const op = errors.Op("activity_plugin_serve")

	errCh := make(chan error, 1)
	var err error
	p.pool, err = p.startPool()
	if err != nil {
		errCh <- errors.E(op, err)
		return errCh
	}

	go func() {
		// single case loop
		for range p.reset {
			if atomic.LoadInt64(&p.closing) == 1 {
				return
			}

			err := p.replacePool()
			if err == nil {
				continue
			}

			bkoff := backoff.NewExponentialBackOff()
			bkoff.InitialInterval = time.Second

			err = backoff.Retry(p.replacePool, bkoff)
			if err != nil {
				errCh <- errors.E(op, err)
			}
		}
	}()

	go func() {
		eventsCh := make(chan events.Event, 10)
		err := p.events.SubscribeP(p.id, "pool.EventWorkerProcessExit", eventsCh)
		if err != nil {
			errCh <- err
			return
		}

		for {
			select {
			case ev := <-eventsCh:
				p.log.Error("Activity pool error", "error", ev.Message())
				p.reset <- struct{}{}
			case <-p.stopCh:
				p.events.Unsubscribe(p.id)
				return
			}
		}
	}()

	return errCh
}

// Stop stops the serving plugin.
func (p *Plugin) Stop() error {
	atomic.StoreInt64(&p.closing, 1)
	const op = errors.Op("activity_plugin_stop")

	pool := p.getPool()
	if pool != nil {
		p.pool = nil
		err := pool.Destroy(context.Background())
		if err != nil {
			return errors.E(op, err)
		}
		return nil
	}

	p.stopCh <- struct{}{}

	return nil
}

// Name of the service.
func (p *Plugin) Name() string {
	return PluginName
}

// RPC returns associated rpc service.
func (p *Plugin) RPC() interface{} {
	return &rpc{srv: p, client: p.temporal.GetClient()}
}

// BaseProcesses returns pool workers.
func (p *Plugin) BaseProcesses() []rrWorker.BaseProcess {
	if p.getPool() == nil {
		return nil
	}
	return p.getPool().Workers()
}

// Workers returns workers process state
func (p *Plugin) Workers() []*process.State {
	if p.getPool() == nil {
		return nil
	}
	workers := p.pool.Workers()
	states := make([]*process.State, 0, len(workers))

	for i := 0; i < len(workers); i++ {
		st, err := process.WorkerProcessState(workers[i])
		if err != nil {
			// log error and continue
			p.log.Error("worker process state error", "error", err)
			continue
		}

		states = append(states, st)
	}

	return states
}

func (p *Plugin) Available() {}

// ActivityNames returns list of all available activities.
func (p *Plugin) ActivityNames() []string {
	return p.pool.ActivityNames()
}

// Reset resets underlying workflow pool with new copy.
func (p *Plugin) Reset() error {
	p.reset <- struct{}{}

	return nil
}

func (p *Plugin) startPool() (activityPool, error) {
	pool, err := newActivityPool(
		p.temporal.GetCodec().WithLogger(p.log),
		p.graceTimeout,
		p.temporal.GetConfig().Activities,
		p.server,
		p.log,
	)

	if err != nil {
		return nil, errors.E(errors.Op("newActivityPool"), err)
	}

	err = pool.Start(context.Background(), p.temporal)
	if err != nil {
		return nil, errors.E(errors.Op("startActivityPool"), err)
	}

	p.log.Debug("Started activity processing", "activities", pool.ActivityNames())

	return pool, nil
}

func (p *Plugin) replacePool() error {
	const op = errors.Op("activity_plugin_replace_pool")
	pool, err := p.startPool()
	if err != nil {
		p.log.Error("replace activity pool failed", "error", err)
		return errors.E(op, err)
	}

	p.log.Debug("replace activity pool")

	var previous activityPool

	p.mu.Lock()
	previous, p.pool = p.pool, pool
	p.mu.Unlock()

	err = previous.Destroy(context.Background())
	if err != nil {
		p.log.Error(
			"Unable to destroy expired activity pool",
			"error",
			errors.E(op, err),
		)
	}

	return nil
}

// getPool returns currently pool.
func (p *Plugin) getPool() activityPool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.pool
}
