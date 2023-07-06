package roadrunner_temporal //nolint:revive,stylecheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/roadrunner-server/endure/v2/dep"
	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v4/events"
	"github.com/roadrunner-server/sdk/v4/metrics"
	"github.com/roadrunner-server/sdk/v4/pool"
	"github.com/roadrunner-server/sdk/v4/state/process"
	"github.com/temporalio/roadrunner-temporal/v4/aggregatedpool"
	"github.com/temporalio/roadrunner-temporal/v4/common"
	"github.com/temporalio/roadrunner-temporal/v4/data_converter"
	"github.com/temporalio/roadrunner-temporal/v4/internal"
	"github.com/temporalio/roadrunner-temporal/v4/internal/codec/proto"
	"github.com/temporalio/roadrunner-temporal/v4/internal/logger"
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
	pluginName string = "temporal"
	metricsKey string = "temporal.metrics"

	// RrMode env variable key
	RrMode string = "RR_MODE"
	// RrCodec env variable key
	RrCodec string = "RR_CODEC"
	// RrCodecVal - codec name, should be in sync with the PHP-SDK
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

type Plugin struct {
	mu sync.RWMutex

	server        common.Server
	log           *zap.Logger
	config        *Config
	statsExporter *metrics.StatsExporter

	mh          temporalClient.MetricsHandler
	tallyCloser io.Closer
	tlsCfg      *tls.Config
	client      temporalClient.Client

	actP  common.Pool
	wfP   common.Pool
	wwPID int

	rrVersion     string
	rrActivityDef *aggregatedpool.Activity
	rrWorkflowDef *aggregatedpool.Workflow
	workflows     map[string]*internal.WorkflowInfo
	activities    map[string]*internal.ActivityInfo
	codec         *proto.Codec

	eventBus events.EventBus
	id       string
	events   chan events.Event
	stopCh   chan struct{}

	workers []worker.Worker

	interceptors map[string]common.Interceptor
}

func (p *Plugin) Init(cfg common.Configurer, log Logger, server common.Server) error {
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

	// CONFIG INIT END -----

	p.log = log.NamedLogger(pluginName)

	p.server = server
	p.rrVersion = cfg.RRVersion()

	// events
	p.events = make(chan events.Event, 1)
	p.eventBus, p.id = events.NewEventBus()
	p.stopCh = make(chan struct{}, 1)
	p.statsExporter = newStatsExporter(p)

	// simple TLS based on the cert and key
	if p.config.TLS != nil {
		var cert tls.Certificate
		var certPool *x509.CertPool
		var rca []byte

		// if client CA is not empty we combine it with Cert and Key
		if p.config.TLS.RootCA != "" {
			cert, err = tls.LoadX509KeyPair(p.config.TLS.Cert, p.config.TLS.Key)
			if err != nil {
				return err
			}

			certPool, err = x509.SystemCertPool()
			if err != nil {
				return err
			}
			if certPool == nil {
				certPool = x509.NewCertPool()
			}

			// we already checked this file in the config.go
			rca, err = os.ReadFile(p.config.TLS.RootCA)
			if err != nil {
				return err
			}

			if ok := certPool.AppendCertsFromPEM(rca); !ok {
				return err
			}

			p.tlsCfg = &tls.Config{
				MinVersion:   tls.VersionTLS12,
				ClientAuth:   p.config.TLS.auth,
				Certificates: []tls.Certificate{cert},
				ClientCAs:    certPool,
				RootCAs:      certPool,
				ServerName:   p.config.TLS.ServerName,
			}
		} else {
			cert, err = tls.LoadX509KeyPair(p.config.TLS.Cert, p.config.TLS.Key)
			if err != nil {
				return err
			}

			p.tlsCfg = &tls.Config{
				ServerName:   p.config.TLS.ServerName,
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{cert},
			}
		}
	}

	// here we need to check
	if p.config.Metrics != nil {
		switch p.config.Metrics.Driver {
		case driverPrometheus:
			ms, cl, err := newPrometheusScope(prometheus.Configuration{
				ListenAddress: p.config.Metrics.Prometheus.Address,
				TimerType:     p.config.Metrics.Prometheus.Type,
			}, p.config.Metrics.Prometheus.Prefix, p.log)
			if err != nil {
				return err
			}

			p.mh = tally.NewMetricsHandler(ms)
			p.tallyCloser = cl
		case driverStatsd:
			ms, cl, err := newStatsdScope(p.config.Metrics.Statsd)
			if err != nil {
				return err
			}
			p.mh = tally.NewMetricsHandler(ms)
			p.tallyCloser = cl
		default:
			return errors.E(op, errors.Errorf("unknown driver provided: %s", p.config.Metrics.Driver))
		}
	}

	p.interceptors = make(map[string]common.Interceptor)

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

func (p *Plugin) Stop(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// stop events
	p.eventBus.Unsubscribe(p.id)
	p.stopCh <- struct{}{}
	p.eventBus = nil

	for i := 0; i < len(p.workers); i++ {
		p.workers[i].Stop()
	}

	// might be nil if the user didn't set the metrics
	if p.tallyCloser != nil {
		err := p.tallyCloser.Close()
		if err != nil {
			return err
		}
	}

	// in case if the Serve func was interrupted
	if p.client != nil {
		p.client.Close()
	}

	return nil
}

func (p *Plugin) Workers() []*process.State {
	p.mu.RLock()
	defer p.mu.RUnlock()

	wfPw := p.wfP.Workers()
	actPw := p.actP.Workers()

	states := make([]*process.State, 0, len(wfPw)+len(actPw))

	for i := 0; i < len(wfPw); i++ {
		st, err := process.WorkerProcessState(wfPw[i])
		if err != nil {
			// log error and continue
			p.log.Error("worker process state error", zap.Error(err))
			continue
		}

		states = append(states, st)
	}

	for i := 0; i < len(actPw); i++ {
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
	wi, err := WorkerInfo(p.codec, p.wfP, p.rrVersion)
	if err != nil {
		return err
	}

	// based on the worker info -> initialize workers
	p.workers, err = aggregatedpool.TemporalWorkers(p.rrWorkflowDef, p.rrActivityDef, wi, p.log, p.client, p.interceptors)
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

	p.activities = ActivitiesInfo(wi)
	p.workflows = WorkflowsInfo(wi)

	return nil
}

// Collects collecting grpc interceptors
func (p *Plugin) Collects() []*dep.In {
	return []*dep.In{
		dep.Fits(func(pp any) {
			mdw := pp.(common.Interceptor)
			// just to be safe
			p.mu.Lock()
			p.interceptors[mdw.Name()] = mdw
			p.mu.Unlock()
		}, (*common.Interceptor)(nil)),
	}
}

func (p *Plugin) Name() string {
	return pluginName
}

func (p *Plugin) RPC() any {
	return &rpc{srv: p, client: p.client}
}

/// INTERNAL

func (p *Plugin) initTemporalClient(phpSdkVersion string, dc converter.DataConverter) error {
	if phpSdkVersion == "" {
		phpSdkVersion = clientBaselineVersion
	}
	p.log.Debug("PHP-SDK version: " + phpSdkVersion)
	worker.SetStickyWorkflowCacheSize(p.config.CacheSize)

	opts := temporalClient.Options{
		HostPort:       p.config.Address,
		MetricsHandler: p.mh,
		Namespace:      p.config.Namespace,
		Logger:         logger.NewZapAdapter(p.log),
		DataConverter:  dc,
		ConnectionOptions: temporalClient.ConnectionOptions{
			TLS: p.tlsCfg,
			DialOptions: []grpc.DialOption{
				grpc.WithUnaryInterceptor(rewriteNameAndVersion(phpSdkVersion)),
			},
		},
	}

	var err error
	p.client, err = temporalClient.Dial(opts)
	if err != nil {
		return err
	}

	p.log.Info("connected to temporal server", zap.String("address", p.config.Address))

	return nil
}

func rewriteNameAndVersion(phpSdkVersion string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		md, _, _ := metadata.FromOutgoingContextRaw(ctx)
		if md == nil {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		md.Set(clientNameHeaderName, clientNameHeaderValue)
		md.Set(clientVersionHeaderName, phpSdkVersion)

		ctx = metadata.NewOutgoingContext(ctx, md)

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func (p *Plugin) initPool() error {
	var err error
	ap, err := p.server.NewPool(context.Background(), p.config.Activities, map[string]string{RrMode: pluginName, RrCodec: RrCodecVal}, p.log)
	if err != nil {
		return err
	}

	dc := data_converter.NewDataConverter(converter.GetDefaultDataConverter())
	p.codec = proto.NewCodec(p.log, dc)

	p.rrActivityDef = aggregatedpool.NewActivityDefinition(p.codec, ap, p.log)

	// ---------- WORKFLOW POOL -------------
	wp, err := p.server.NewPool(
		context.Background(),
		&pool.Config{
			NumWorkers:      1,
			Command:         p.config.Activities.Command,
			AllocateTimeout: time.Hour * 240,
			DestroyTimeout:  time.Second * 30,
			// no supervisor for the workflow worker
			Supervisor: nil,
		},
		map[string]string{RrMode: pluginName, RrCodec: RrCodecVal},
		nil,
	)
	if err != nil {
		return err
	}

	p.rrWorkflowDef = aggregatedpool.NewWorkflowDefinition(p.codec, wp, p.log)

	// get worker information
	wi, err := WorkerInfo(p.codec, wp, p.rrVersion)
	if err != nil {
		return err
	}

	if len(wi) == 0 {
		return errors.Str("worker info should contain at least 1 worker")
	}

	err = p.initTemporalClient(wi[0].PhpSdkVersion, dc)
	if err != nil {
		return err
	}

	p.workers, err = aggregatedpool.TemporalWorkers(p.rrWorkflowDef, p.rrActivityDef, wi, p.log, p.client, p.interceptors)
	if err != nil {
		return err
	}

	for i := 0; i < len(p.workers); i++ {
		err = p.workers[i].Start()
		if err != nil {
			return err
		}
	}

	p.activities = ActivitiesInfo(wi)
	p.workflows = WorkflowsInfo(wi)
	p.actP = ap
	p.wfP = wp

	if len(p.wfP.Workers()) < 1 {
		return errors.E(errors.Str("failed to allocate a workflow worker"))
	}

	// we have only 1 worker for the workflow pool
	p.wwPID = int(p.wfP.Workers()[0].Pid())

	return nil
}
