package roadrunner_temporal //nolint:revive,stylecheck

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

	"github.com/roadrunner-server/api/v2/event_bus"
	"github.com/roadrunner-server/api/v2/plugins/config"
	"github.com/roadrunner-server/api/v2/plugins/server"
	rrPool "github.com/roadrunner-server/api/v2/pool"
	"github.com/roadrunner-server/api/v2/state/process"
	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v2/events"
	"github.com/roadrunner-server/sdk/v2/metrics"
	poolImpl "github.com/roadrunner-server/sdk/v2/pool"
	processImpl "github.com/roadrunner-server/sdk/v2/state/process"
	"github.com/temporalio/roadrunner-temporal/aggregatedpool"
	"github.com/temporalio/roadrunner-temporal/data_converter"
	"github.com/temporalio/roadrunner-temporal/internal"
	"github.com/temporalio/roadrunner-temporal/internal/codec/proto"
	"github.com/temporalio/roadrunner-temporal/internal/logger"
	"github.com/uber-go/tally/v4/prometheus"
	temporalClient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/tally"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/worker"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	// PluginName defines public service name.
	PluginName string = "temporal"
	metricsKey string = "temporal.metrics"

	// RrMode env variable key
	RrMode string = "RR_MODE"

	// RrCodec env variable key
	RrCodec string = "RR_CODEC"

	// RrCodecVal - codec name, should be in sync with the PHP-SDK
	RrCodecVal string = "protobuf"

	// temporal, sync with https://github.com/temporalio/sdk-go/blob/master/internal/internal_utils.go#L44
	clientNameHeaderName     = "client-name"
	clientNameHeaderValue    = "roadrunner-temporal"
	clientVersionHeaderName  = "client-version"
	clientVersionHeaderValue = "1.5.1"
)

type Plugin struct {
	mu sync.RWMutex

	server        server.Server
	log           *zap.Logger
	config        *Config
	tallyCloser   io.Closer
	statsExporter *metrics.StatsExporter

	client        temporalClient.Client
	dataConverter converter.DataConverter

	actP  rrPool.Pool
	wfP   rrPool.Pool
	wwPID int

	rrVersion     string
	rrActivityDef *aggregatedpool.Activity
	rrWorkflowDef *aggregatedpool.Workflow
	workflows     map[string]*internal.WorkflowInfo
	activities    map[string]*internal.ActivityInfo
	codec         *proto.Codec

	eventBus event_bus.EventBus
	id       string
	events   chan event_bus.Event
	stopCh   chan struct{}

	seqID        uint64
	workers      []worker.Worker
	graceTimeout time.Duration
}

func (p *Plugin) Init(cfg config.Configurer, log *zap.Logger, server server.Server) error {
	const op = errors.Op("temporal_plugin_init")

	if !cfg.Has(PluginName) {
		return errors.E(op, errors.Disabled)
	}

	err := cfg.UnmarshalKey(PluginName, &p.config)
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

	p.dataConverter = data_converter.NewDataConverter(converter.GetDefaultDataConverter())
	p.log = &zap.Logger{}
	*p.log = *log

	p.server = server
	p.graceTimeout = cfg.GracefulTimeout()
	p.rrVersion = cfg.RRVersion()

	// events
	p.events = make(chan event_bus.Event, 1)
	p.eventBus, p.id = events.Bus()
	p.stopCh = make(chan struct{}, 1)
	p.statsExporter = newStatsExporter(p)

	return nil
}

func (p *Plugin) Serve() chan error {
	errCh := make(chan error, 1)
	const op = errors.Op("temporal_plugin_serve")

	p.mu.Lock()
	defer p.mu.Unlock()

	worker.SetStickyWorkflowCacheSize(p.config.CacheSize)

	opts := temporalClient.Options{
		HostPort:      p.config.Address,
		Namespace:     p.config.Namespace,
		Logger:        logger.NewZapAdapter(p.log),
		DataConverter: p.dataConverter,
		ConnectionOptions: temporalClient.ConnectionOptions{
			DialOptions: []grpc.DialOption{
				grpc.WithUnaryInterceptor(rewriteNameAndVersion),
			},
		},
	}

	// simple TLS based on the cert and key
	if p.config.TLS != nil {
		cert, err := tls.LoadX509KeyPair(p.config.TLS.Cert, p.config.TLS.Key)
		if err != nil {
			errCh <- err
			return errCh
		}

		opts.ConnectionOptions.TLS = &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		}
	}

	/*
		TODO(rustatian): simplify
		set up metrics handler
	*/
	if p.config.Metrics != nil {
		switch p.config.Metrics.Driver {
		case driverPrometheus:
			ms, cl, errPs := newPrometheusScope(prometheus.Configuration{
				ListenAddress: p.config.Metrics.Prometheus.Address,
				TimerType:     p.config.Metrics.Prometheus.Type,
			}, p.config.Metrics.Prometheus.Prefix, p.log)
			if errPs != nil {
				errCh <- errors.E(op, errPs)
				return errCh
			}

			opts.MetricsHandler = tally.NewMetricsHandler(ms)
			p.tallyCloser = cl
		case driverStatsd:
			ms, cl, errSt := newStatsdScope(p.config.Metrics.Statsd)
			if errSt != nil {
				errCh <- errSt
				return errCh
			}
			opts.MetricsHandler = tally.NewMetricsHandler(ms)
			p.tallyCloser = cl
		default:
			errCh <- errors.E(op, errors.Errorf("unknown driver provided: %s", p.config.Metrics.Driver))
			return errCh
		}
	}

	var err error
	p.client, err = temporalClient.Dial(opts)
	if err != nil {
		errCh <- errors.E(op, err)
		return errCh
	}

	p.log.Info("connected to temporal server", zap.String("address", p.config.Address))
	p.codec = proto.NewCodec(p.log, p.dataConverter)

	err = p.initPool()
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

func (p *Plugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// stop events
	p.eventBus.Unsubscribe(p.id)
	p.stopCh <- struct{}{}
	p.eventBus = nil

	for i := 0; i < len(p.workers); i++ {
		p.workers[i].Stop()
	}

	if p.tallyCloser != nil {
		err := p.tallyCloser.Close()
		if err != nil {
			return err
		}
	}

	if p.client != nil {
		p.client.Close()
	}

	return nil
}

func (p *Plugin) Workers() []*process.State {
	p.mu.RLock()
	wfPw := p.wfP.Workers()
	actPw := p.actP.Workers()
	p.mu.RUnlock()

	states := make([]*process.State, 0, len(wfPw)+len(actPw))

	for i := 0; i < len(wfPw); i++ {
		st, err := processImpl.WorkerProcessState(wfPw[i])
		if err != nil {
			// log error and continue
			p.log.Error("worker process state error", zap.Error(err))
			continue
		}

		states = append(states, st)
	}

	for i := 0; i < len(actPw); i++ {
		st, err := processImpl.WorkerProcessState(actPw[i])
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
	for i := 0; i < len(p.workers); i++ {
		p.workers[i].Stop()
	}

	p.workers = nil
	worker.PurgeStickyWorkflowCache()

	ctxW, cancelW := context.WithTimeout(context.Background(), time.Second*30)
	defer cancelW()
	errWp := p.wfP.Reset(ctxW)
	if errWp != nil {
		return errors.E(op, errWp)
	}
	p.log.Info("workflow pool restarted")

	ctxA, cancelA := context.WithTimeout(context.Background(), time.Second*30)
	defer cancelA()
	errAp := p.actP.Reset(ctxA)
	if errAp != nil {
		return errors.E(op, errAp)
	}
	p.log.Info("activity pool restarted")

	// get worker info
	wi := make([]*internal.WorkerInfo, 0, 5)
	err := aggregatedpool.GetWorkerInfo(p.codec, p.wfP, p.rrVersion, &wi)
	if err != nil {
		return err
	}

	// based on the worker info -> initialize workers
	p.workers, err = aggregatedpool.InitWorkers(p.rrWorkflowDef, p.rrActivityDef, wi, p.log, p.client, p.graceTimeout)
	if err != nil {
		return err
	}

	// start workers
	for i := 0; i < len(p.workers); i++ {
		err = p.workers[i].Start()
		if err != nil {
			return err
		}
	}

	p.activities = aggregatedpool.GrabActivities(wi)
	p.workflows = aggregatedpool.GrabWorkflows(wi)

	return nil
}

func (p *Plugin) Name() string {
	return PluginName
}

func (p *Plugin) RPC() any {
	return &rpc{srv: p, client: p.client}
}

func (p *Plugin) SedID() uint64 {
	p.log.Debug("sequenceID", zap.Uint64("before", atomic.LoadUint64(&p.seqID)))
	defer p.log.Debug("sequenceID", zap.Uint64("after", atomic.LoadUint64(&p.seqID)+1))
	return atomic.AddUint64(&p.seqID, 1)
}

func rewriteNameAndVersion(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	md, _, _ := metadata.FromOutgoingContextRaw(ctx)
	if md == nil {
		return nil
	}

	md.Set(clientNameHeaderName, clientNameHeaderValue)
	md.Set(clientVersionHeaderName, clientVersionHeaderValue)

	ctx = metadata.NewOutgoingContext(ctx, md)

	return invoker(ctx, method, req, reply, cc, opts...)
}

func (p *Plugin) initPool() error {
	var err error
	ap, err := p.server.NewWorkerPool(context.Background(), p.config.Activities, map[string]string{RrMode: PluginName, RrCodec: RrCodecVal}, p.log)
	if err != nil {
		return err
	}

	p.rrActivityDef = aggregatedpool.NewActivityDefinition(p.codec, ap, p.log, p.dataConverter, p.client, p.graceTimeout)

	// ---------- WORKFLOW POOL -------------
	wp, err := p.server.NewWorkerPool(
		context.Background(),
		&poolImpl.Config{
			NumWorkers:      1,
			Command:         p.config.Activities.Command,
			AllocateTimeout: time.Hour * 240,
			DestroyTimeout:  time.Second * 30,
			// no supervisor for the workflow worker
			Supervisor: nil,
		},
		map[string]string{RrMode: PluginName, RrCodec: RrCodecVal},
		nil,
	)
	if err != nil {
		return err
	}

	p.rrWorkflowDef = aggregatedpool.NewWorkflowDefinition(p.codec, p.dataConverter, wp, p.log, p.SedID, p.client, p.graceTimeout)

	// get worker information
	wi := make([]*internal.WorkerInfo, 0, 5)
	err = aggregatedpool.GetWorkerInfo(p.codec, wp, p.rrVersion, &wi)
	if err != nil {
		return err
	}

	p.workers, err = aggregatedpool.InitWorkers(p.rrWorkflowDef, p.rrActivityDef, wi, p.log, p.client, p.graceTimeout)
	if err != nil {
		return err
	}

	for i := 0; i < len(p.workers); i++ {
		err = p.workers[i].Start()
		if err != nil {
			return err
		}
	}

	p.activities = aggregatedpool.GrabActivities(wi)
	p.workflows = aggregatedpool.GrabWorkflows(wi)
	p.actP = ap
	p.wfP = wp

	if len(p.wfP.Workers()) < 1 {
		return errors.E(errors.Str("failed to allocate a workflow worker"))
	}

	// we have only 1 worker for the workflow pool
	p.wwPID = int(p.wfP.Workers()[0].Pid())

	return nil
}
