package rrtemporal

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/roadrunner-server/endure/v2/dep"
	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/events"
	"github.com/roadrunner-server/pool/state/process"
	"github.com/temporalio/roadrunner-temporal/v5/aggregatedpool"
	"github.com/temporalio/roadrunner-temporal/v5/api"
	"github.com/temporalio/roadrunner-temporal/v5/internal"
	"github.com/temporalio/roadrunner-temporal/v5/internal/codec/proto"
	tclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.uber.org/zap"

	"github.com/roadrunner-server/pool/pool/static_pool"
)

const (
	// PluginName defines public service name.
	pluginName string = "temporal"
	metricsKey string = "temporal.metrics"

	// RrMode env variable key
	RrMode string = "RR_MODE"
	// RrCodec env variable key
	RrCodec string = "RR_CODEC"
	// RrCodecVal - codec name should be in sync with the PHP-SDK
	RrCodecVal string = "protobuf"

	// temporal, sync with https://github.com/temporalio/sdk-go/blob/master/internal/internal_utils.go#L44
	clientNameHeaderName    = "client-name"
	clientNameHeaderValue   = "temporal-php-2"
	clientVersionHeaderName = "client-version"
	clientBaselineVersion   = "2.5.0"
)

type Logger interface {
	NamedLogger(name string) *zap.Logger
}

// temporal structure contains temporal specific structures
type temporal struct {
	rrActivityDef *aggregatedpool.Activity
	rrWorkflowDef *aggregatedpool.Workflow
	workflows     map[string]*internal.WorkflowInfo
	activities    map[string]*internal.ActivityInfo
	mh            tclient.MetricsHandler
	tallyCloser   io.Closer
	tlsCfg        *tls.Config
	client        tclient.Client
	workers       []worker.Worker

	interceptors map[string]api.Interceptor
}

type Plugin struct {
	mu sync.RWMutex

	server        api.Server
	log           *zap.Logger
	config        *Config
	statsExporter *StatsExporter
	codec         *proto.Codec
	actP          *static_pool.Pool
	wfP           *static_pool.Pool
	// updated from the PHP SDK
	apiKey atomic.Pointer[string]

	id        string
	wwPID     int
	rrVersion string
	temporal  *temporal

	eventBus events.EventBus
	events   chan events.Event
	stopCh   chan struct{}
}

func (p *Plugin) Init(cfg api.Configurer, log Logger, server api.Server) error {
	const op = errors.Op("temporal_plugin_init")

	if !cfg.Has(pluginName) {
		return errors.E(op, errors.Disabled)
	}

	err := cfg.UnmarshalKey(pluginName, &p.config)
	if err != nil {
		return errors.E(op, err)
	}

	/*
		Parse metrics configuration
		default (no BC): prometheus
	*/
	if p.config.Metrics != nil {
		switch p.config.Metrics.Driver {
		case driverPrometheus:
			err = cfg.UnmarshalKey(metricsKey, &p.config.Metrics.Prometheus)
			if err != nil {
				return errors.E(op, err)
			}
		case driverStatsd:
			err = cfg.UnmarshalKey(metricsKey, &p.config.Metrics.Statsd)
			if err != nil {
				return errors.E(op, err)
			}
		default:
			err = cfg.UnmarshalKey(metricsKey, &p.config.Metrics.Prometheus)
			if err != nil {
				return errors.E(op, err)
			}
		}
	}

	err = p.config.InitDefault()
	if err != nil {
		return errors.E(op, err)
	}
	// init temporal section
	p.temporal = &temporal{}
	// CONFIG INIT END -----

	p.log = log.NamedLogger(pluginName)

	p.server = server
	p.rrVersion = cfg.RRVersion()

	// events
	p.events = make(chan events.Event, 1)
	p.eventBus, p.id = events.NewEventBus()
	p.stopCh = make(chan struct{}, 1)
	p.statsExporter = newStatsExporter(p)

	// initialize TLS
	if p.config.TLS != nil {
		p.temporal.tlsCfg, err = initTLS(p.config)
		if err != nil {
			return errors.E(op, err)
		}
	}

	// here we need to check
	if p.config.Metrics != nil {
		p.temporal.mh, p.temporal.tallyCloser, err = initMetrics(p.config, p.log)
		if err != nil {
			return errors.E(op, err)
		}
	}

	// initialize interceptors
	p.temporal.interceptors = make(map[string]api.Interceptor)
	// empty
	p.apiKey.Store(ptrTo(""))

	return nil
}

func (p *Plugin) Serve() chan error {
	errCh := make(chan error, 1)
	const op = errors.Op("temporal_plugin_serve")

	p.mu.Lock()
	defer p.mu.Unlock()

	err := p.initPool()
	if err != nil {
		errCh <- errors.E(op, err)
		return errCh
	}

	err = p.eventBus.SubscribeP(p.id, fmt.Sprintf("*.%s", events.EventWorkerStopped.String()), p.events)
	if err != nil {
		errCh <- errors.E(op, err)
		return errCh
	}

	go func() {
		for {
			select {
			case ev := <-p.events:
				p.log.Debug("worker stopped, restarting pool and temporal workers", zap.String("message", ev.Message()))

				// check pid, message from the go sdk is: process exited, pid: 334455 <-- we are looking for this pid
				// sdk 2.18.1
				// TODO: potential bug here, if the pid contains the WW pid, it will reset everything (btw, should not be a problem)
				switch strings.Contains(ev.Message(), strconv.Itoa(p.wwPID)) {
				// stopped workflow worker
				case true:
					errR := p.Reset()
					if errR != nil {
						errCh <- errors.E(op, errors.Errorf("error during reset: %#v, event: %s", errR, ev.Message()))
						return
					}
					// stopped one of the activity workers
				case false:
					errR := p.ResetAP()
					if errR != nil {
						errCh <- errors.E(op, errors.Errorf("error during reset: %#v, event: %s", errR, ev.Message()))
						return
					}
				}

			case <-p.stopCh:
				return
			}
		}
	}()

	return errCh
}

func (p *Plugin) Stop(ctx context.Context) error {
	doneCh := make(chan struct{}, 1)

	go func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		// stop events
		p.eventBus.Unsubscribe(p.id)
		p.stopCh <- struct{}{}
		p.eventBus = nil

		// destroy worker pools
		// WP
		if p.wfP != nil {
			p.wfP.Destroy(ctx)
		}

		// ACT pool
		if p.actP != nil {
			p.actP.Destroy(ctx)
		}

		// stop receiving tasks
		for i := 0; i < len(p.temporal.workers); i++ {
			p.temporal.workers[i].Stop()
		}

		// might be nil if the user didn't set the metrics
		if p.temporal.tallyCloser != nil {
			// there might be a panic if the io.Closer is not nil, but the actual type is nil
			err := p.temporal.tallyCloser.Close()
			if err != nil {
				p.mu.Unlock()
			}
		}

		// in case if the Serve func was interrupted
		if p.temporal.client != nil {
			p.temporal.client.Close()
		}

		doneCh <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneCh:
		return nil
	}
}

func (p *Plugin) Workers() []*process.State {
	p.mu.RLock()
	defer p.mu.RUnlock()

	wfPw := p.wfP.Workers()
	actPw := p.actP.Workers()

	states := make([]*process.State, 0, len(wfPw)+len(actPw))

	for i := range wfPw {
		st, err := process.WorkerProcessState(wfPw[i])
		if err != nil {
			// log error and continue
			p.log.Error("worker process state error", zap.Error(err))
			continue
		}

		states = append(states, st)
	}

	for i := range actPw {
		st, err := process.WorkerProcessState(actPw[i])
		if err != nil {
			// log error and continue
			p.log.Error("worker process state error", zap.Error(err))
			continue
		}

		states = append(states, st)
	}

	return states
}

func (p *Plugin) ResetAP() error {
	const op = errors.Op("temporal_plugin_reset")

	p.mu.Lock()
	defer p.mu.Unlock()

	p.log.Info("reset signal received, resetting activity pool")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	errAp := p.actP.Reset(ctx)
	if errAp != nil {
		return errors.E(op, errAp)
	}
	p.log.Info("activity pool restarted")

	return nil
}

func (p *Plugin) Reset() error {
	const op = errors.Op("temporal_reset")

	p.mu.Lock()
	defer p.mu.Unlock()

	p.log.Info("reset signal received, resetting activity and workflow worker pools")

	// stop temporal workers
	for i := 0; i < len(p.temporal.workers); i++ {
		p.temporal.workers[i].Stop()
	}

	p.temporal.workers = nil
	worker.PurgeStickyWorkflowCache()

	if len(p.wfP.Workers()) < 1 {
		return errors.E(op, errors.Str("failed to allocate a workflow worker"))
	}

	p.wwPID = int(p.wfP.Workers()[0].Pid())

	ctxA, cancelA := context.WithTimeout(context.Background(), time.Second*30)
	defer cancelA()
	errAp := p.actP.Reset(ctxA)
	if errAp != nil {
		return errors.E(op, errAp)
	}
	p.log.Info("activity pool restarted")

	// get worker info
	wi, err := WorkerInfo(p.codec, p.wfP, p.rrVersion, p.wwPID)
	if err != nil {
		return err
	}

	// based on the worker info -> initialize workers
	workers, err := aggregatedpool.TemporalWorkers(
		p.temporal.rrWorkflowDef,
		p.temporal.rrActivityDef,
		wi,
		p.log,
		p.temporal.client,
		p.temporal.interceptors,
	)
	if err != nil {
		return err
	}

	// start workers
	for i := range workers {
		err = workers[i].Start()
		if err != nil {
			return err
		}
	}

	p.temporal.activities = ActivitiesInfo(wi)
	p.temporal.workflows = WorkflowsInfo(wi)
	p.temporal.workers = workers

	return nil
}

// Collects collecting grpc interceptors
func (p *Plugin) Collects() []*dep.In {
	return []*dep.In{
		dep.Fits(func(pp any) {
			mdw := pp.(api.Interceptor)
			// just to be safe
			p.mu.Lock()
			p.temporal.interceptors[mdw.Name()] = mdw
			p.mu.Unlock()
		}, (*api.Interceptor)(nil)),
	}
}

func (p *Plugin) Name() string {
	return pluginName
}

func (p *Plugin) RPC() any {
	return &rpc{plugin: p, client: p.temporal.client}
}

func ptrTo[T any](v T) *T {
	return &v
}
