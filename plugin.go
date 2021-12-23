package roadrunner_temporal //nolint:revive,stylecheck

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spiral/errors"
	"github.com/spiral/roadrunner-plugins/v2/config"
	"github.com/spiral/roadrunner-plugins/v2/logger"
	"github.com/spiral/roadrunner-plugins/v2/server"
	rrPool "github.com/spiral/roadrunner/v2/pool"
	"github.com/spiral/roadrunner/v2/state/process"
	temporalClient "github.com/spiral/sdk-go/client"
	"github.com/spiral/sdk-go/contrib/tally"
	"github.com/spiral/sdk-go/converter"
	"github.com/spiral/sdk-go/worker"
	"github.com/temporalio/roadrunner-temporal/activity"
	"github.com/temporalio/roadrunner-temporal/internal"
	"github.com/temporalio/roadrunner-temporal/internal/codec/proto"
	"github.com/temporalio/roadrunner-temporal/internal/data_converter"
	"github.com/temporalio/roadrunner-temporal/workflow"
	"github.com/uber-go/tally/v4/prometheus"
)

// PluginName defines public service name.
const (
	PluginName string = "temporal"

	// RrMode env variable key
	RrMode string = "RR_MODE"

	// RrCodec env variable key
	RrCodec string = "RR_CODEC"

	RrCodecVal string = "protobuf"
)

type Plugin struct {
	sync.RWMutex

	server      server.Server
	log         logger.Logger
	config      *Config
	tallyCloser io.Closer

	client        temporalClient.Client
	dataConverter converter.DataConverter

	actP rrPool.Pool
	wfP  rrPool.Pool

	rrActivity internal.Activity
	rrWorkflow internal.Workflow

	seqID uint64

	actW []worker.Worker
	wfW  []worker.Worker

	graceTimeout time.Duration
}

func (p *Plugin) Init(cfg config.Configurer, log logger.Logger, server server.Server) error {
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
	p.graceTimeout = cfg.GetCommonConfig().GracefulTimeout

	return nil
}

func (p *Plugin) Serve() chan error {
	errCh := make(chan error, 1)
	const op = errors.Op("temporal_serve")

	p.Lock()
	defer p.Unlock()

	env := map[string]string{RrMode: PluginName, RrCodec: RrCodecVal}
	var err error
	opts := temporalClient.Options{
		HostPort:      p.config.Address,
		Namespace:     p.config.Namespace,
		Logger:        p.log,
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

	worker.SetStickyWorkflowCacheSize(p.config.CacheSize)

	p.log.Info("connected to temporal server", "address", p.config.Address)

	pl, err := p.server.NewWorkerPool(context.Background(), p.config.Activities, env)
	if err != nil {
		errCh <- errors.E(op, err)
		return errCh
	}

	activityCodec := proto.NewCodec(pl, p.log, p.dataConverter)
	ap := activity.NewActivityDefinition(activityCodec, p.log, p.dataConverter, p.client, p.graceTimeout)
	p.actW, err = ap.Init()
	if err != nil {
		errCh <- errors.E(op, err)
		return errCh
	}

	// ---------- WORKFLOWS -------------
	wpl, err := p.server.NewWorkerPool(
		context.Background(),
		&rrPool.Config{
			NumWorkers:      1,
			AllocateTimeout: time.Hour * 240,
			DestroyTimeout:  time.Second * 30,
			// no supervisor for the workflow worker
			Supervisor: nil,
		},
		env,
	)
	if err != nil {
		errCh <- err
		return errCh
	}

	wfCodec := proto.NewCodec(wpl, p.log, p.dataConverter)
	// can be 2+ workflow workers
	wfDef := workflow.NewWorkflowDefinition(wfCodec, p.log, p.SedID, p.client, p.graceTimeout)
	p.wfW, err = wfDef.Init()
	if err != nil {
		errCh <- err
		return errCh
	}

	for i := 0; i < len(p.wfW); i++ {
		err = p.wfW[i].Start()
		if err != nil {
			errCh <- err
			return errCh
		}
	}
	for i := 0; i < len(p.actW); i++ {
		err = p.actW[i].Start()
		if err != nil {
			errCh <- err
			return errCh
		}
	}

	p.rrActivity = ap
	p.rrWorkflow = wfDef

	p.actP = pl
	p.wfP = wpl

	return errCh
}

func (p *Plugin) Stop() error {
	p.Lock()
	defer p.Unlock()

	for i := 0; i < len(p.actW); i++ {
		p.actW[i].Stop()
	}

	p.actP.Destroy(context.Background())

	for i := 0; i < len(p.wfW); i++ {
		p.wfW[i].Stop()
	}

	p.wfP.Destroy(context.Background())

	if p.client != nil {
		p.client.Close()
	}

	if p.tallyCloser != nil {
		err := p.tallyCloser.Close()
		if err != nil {
			return err
		}
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
		st, err := process.WorkerProcessState(wfPw[i])
		if err != nil {
			// log error and continue
			p.log.Error("worker process state error", "error", err)
			continue
		}

		states = append(states, st)
	}

	for i := 0; i < len(actPw); i++ {
		st, err := process.WorkerProcessState(actPw[i])
		if err != nil {
			// log error and continue
			p.log.Error("worker process state error", "error", err)
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
	p.log.Debug("sequenceID", "before", atomic.LoadUint64(&p.seqID))
	defer p.log.Debug("sequenceID", "after", atomic.LoadUint64(&p.seqID)+1)
	return atomic.AddUint64(&p.seqID, 1)
}
