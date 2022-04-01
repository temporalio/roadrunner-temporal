package roadrunner_temporal //nolint:revive,stylecheck

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/roadrunner-server/api/v2/plugins/config"
	"github.com/roadrunner-server/api/v2/plugins/server"
	rrPool "github.com/roadrunner-server/api/v2/pool"
	"github.com/roadrunner-server/api/v2/state/process"
	"github.com/roadrunner-server/errors"
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
)

const (
	// PluginName defines public service name.
	PluginName string = "temporal"

	// RrMode env variable key
	RrMode string = "RR_MODE"

	// RrCodec env variable key
	RrCodec string = "RR_CODEC"

	// RrCodecVal - codec name, should be in sync with the PHP-SDK
	RrCodecVal string = "protobuf"
)

type Plugin struct {
	sync.RWMutex

	server      server.Server
	log         *zap.Logger
	config      *Config
	tallyCloser io.Closer

	client        temporalClient.Client
	dataConverter converter.DataConverter

	actP rrPool.Pool
	wfP  rrPool.Pool

	rrActivity *aggregatedpool.Activity
	rrWorkflow *aggregatedpool.Workflow
	workflows  map[string]internal.WorkflowInfo
	activities []string

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

	p.config.InitDefault()

	p.dataConverter = data_converter.NewDataConverter(converter.GetDefaultDataConverter())
	p.log = log
	p.server = server
	p.graceTimeout = cfg.GracefulTimeout()

	return nil
}

func (p *Plugin) Serve() chan error {
	errCh := make(chan error, 1)
	const op = errors.Op("temporal_serve")

	p.Lock()
	defer p.Unlock()

	worker.SetStickyWorkflowCacheSize(p.config.CacheSize)

	env := map[string]string{RrMode: PluginName, RrCodec: RrCodecVal}
	var err error
	opts := temporalClient.Options{
		HostPort:      p.config.Address,
		Namespace:     p.config.Namespace,
		Logger:        logger.NewZapAdapter(p.log),
		DataConverter: p.dataConverter,
	}

	if p.config.Metrics != nil {
		ms, cl, errPs := newPrometheusScope(prometheus.Configuration{
			ListenAddress: p.config.Metrics.Address,
			TimerType:     p.config.Metrics.Type,
		}, p.config.Metrics.Prefix, p.log)
		if errPs != nil {
			errCh <- errors.E(op, errPs)
			return errCh
		}

		opts.MetricsHandler = tally.NewMetricsHandler(ms)
		p.tallyCloser = cl
	}

	p.client, err = temporalClient.NewClient(opts)
	if err != nil {
		errCh <- err
		return errCh
	}

	p.log.Info("connected to temporal server", zap.String("address", p.config.Address))
	codec := proto.NewCodec(p.log, p.dataConverter)

	// ------ ACTIVITIES POOL --------
	pl, err := p.server.NewWorkerPool(context.Background(), p.config.Activities, env, nil)
	if err != nil {
		errCh <- errors.E(op, err)
		return errCh
	}

	// init codec
	ap := aggregatedpool.NewActivityDefinition(codec, pl, p.log, p.dataConverter, p.client, p.graceTimeout)
	// --------------------------------

	// ---------- WORKFLOWS -------------
	wpl, err := p.server.NewWorkerPool(
		context.Background(),
		&poolImpl.Config{
			NumWorkers:      1,
			AllocateTimeout: time.Hour * 240,
			DestroyTimeout:  time.Second * 30,
			// no supervisor for the workflow worker
			Supervisor: nil,
		},
		env,
		nil,
	)
	if err != nil {
		errCh <- err
		return errCh
	}

	wfDef := aggregatedpool.NewWorkflowDefinition(codec, p.dataConverter, wpl, p.log, p.SedID, p.client, p.graceTimeout)

	var workers []worker.Worker
	workers, p.workflows, p.activities, err = aggregatedpool.Init(wfDef, ap, wpl, codec, p.log, p.client, p.graceTimeout)
	if err != nil {
		return nil
	}

	// --------------------------------

	// ---------- START WORKERS ---------------
	for i := 0; i < len(workers); i++ {
		err = workers[i].Start()
		if err != nil {
			errCh <- err
			return errCh
		}
	}

	// --------------------------------

	p.rrActivity = ap
	p.rrWorkflow = wfDef
	p.workers = workers
	p.actP = pl
	p.wfP = wpl

	return errCh
}

func (p *Plugin) Stop() error {
	p.Lock()
	defer p.Unlock()

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
	p.RLock()
	wfPw := p.wfP.Workers()
	actPw := p.actP.Workers()
	p.RUnlock()

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

func (p *Plugin) Reset() error {
	const op = errors.Op("temporal_reset")
	p.Lock()
	defer p.Unlock()

	p.log.Info("reset signal received, resetting activity and workflow worker pools")

	err := p.wfP.Reset(context.Background())
	if err != nil {
		return errors.E(op, err)
	}
	p.log.Info("workflow pool restarted")

	err = p.actP.Reset(context.Background())
	if err != nil {
		return errors.E(op, err)
	}
	p.log.Info("activity pool restarted")

	return nil
}

func (p *Plugin) Name() string {
	return PluginName
}

func (p *Plugin) RPC() interface{} {
	return &rpc{srv: p, client: p.client}
}

func (p *Plugin) SedID() uint64 {
	p.log.Debug("sequenceID", zap.Uint64("before", atomic.LoadUint64(&p.seqID)))
	defer p.log.Debug("sequenceID", zap.Uint64("after", atomic.LoadUint64(&p.seqID)+1))
	return atomic.AddUint64(&p.seqID, 1)
}
