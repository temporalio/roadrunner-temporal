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
	"sync/atomic"
	"time"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v3/events"
	"github.com/roadrunner-server/sdk/v3/metrics"
	"github.com/roadrunner-server/sdk/v3/pool"
	"github.com/roadrunner-server/sdk/v3/state/process"
	"github.com/temporalio/roadrunner-temporal/v2/aggregatedpool"
	"github.com/temporalio/roadrunner-temporal/v2/common"
	"github.com/temporalio/roadrunner-temporal/v2/data_converter"
	"github.com/temporalio/roadrunner-temporal/v2/internal"
	"github.com/temporalio/roadrunner-temporal/v2/internal/codec/proto"
	"github.com/temporalio/roadrunner-temporal/v2/internal/logger"
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
	clientVersionHeaderValue = "2.0.0"
)

type Plugin struct {
	mu sync.RWMutex

	server        common.Server
	log           *zap.Logger
	config        *Config
	tallyCloser   io.Closer
	statsExporter *metrics.StatsExporter

	client        temporalClient.Client
	dataConverter converter.DataConverter

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

	seqID        uint64
	workers      []worker.Worker
	graceTimeout time.Duration
}

func (p *Plugin) Init(cfg common.Configurer, log *zap.Logger, server common.Server) error {
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
	p.events = make(chan events.Event, 1)
	p.eventBus, p.id = events.NewEventBus()
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
		var cert tls.Certificate
		var certPool *x509.CertPool
		var rca []byte
		var err error

		// if client CA is not empty we combine it with Cert and Key
		if p.config.TLS.RootCA != "" {
			cert, err = tls.LoadX509KeyPair(p.config.TLS.Cert, p.config.TLS.Key)
			if err != nil {
				errCh <- errors.E(op, err)
				return errCh
			}

			certPool, err = x509.SystemCertPool()
			if err != nil {
				errCh <- errors.E(op, err)
				return errCh
			}
			if certPool == nil {
				certPool = x509.NewCertPool()
			}

			// we already checked this file in the config.go
			rca, err = os.ReadFile(p.config.TLS.RootCA)
			if err != nil {
				errCh <- errors.E(op, err)
				return errCh
			}

			if ok := certPool.AppendCertsFromPEM(rca); !ok {
				errCh <- errors.E(op, errors.Str("could not append Certs from PEM"))
				return errCh
			}

			opts.ConnectionOptions.TLS = &tls.Config{
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
				errCh <- err
				return errCh
			}

			opts.ConnectionOptions.TLS = &tls.Config{
				ServerName:   p.config.TLS.ServerName,
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{cert},
			}
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
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	md.Set(clientNameHeaderName, clientNameHeaderValue)
	md.Set(clientVersionHeaderName, clientVersionHeaderValue)

	ctx = metadata.NewOutgoingContext(ctx, md)

	return invoker(ctx, method, req, reply, cc, opts...)
}

func (p *Plugin) initPool() error {
	var err error
	ap, err := p.server.NewPool(context.Background(), p.config.Activities, map[string]string{RrMode: PluginName, RrCodec: RrCodecVal}, p.log)
	if err != nil {
		return err
	}

	p.rrActivityDef = aggregatedpool.NewActivityDefinition(p.codec, ap, p.log, p.dataConverter, p.client, p.graceTimeout)

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
