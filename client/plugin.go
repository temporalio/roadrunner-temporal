package client

import (
	"fmt"
	"io"
	"os"
	"sync/atomic"

	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/plugins/config"
	"github.com/spiral/roadrunner/v2/plugins/logger"
	rrt "github.com/temporalio/roadrunner-temporal/protocol"
	"github.com/uber-go/tally"
	"github.com/uber-go/tally/prometheus"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/worker"
)

// PluginName defines public service name.
const PluginName = "temporal"

// Plugin implement Temporal contract.
type Plugin struct {
	workerID    int32
	cfg         *Config
	dc          converter.DataConverter
	log         logger.Logger
	client      client.Client
	tallyCloser io.Closer
}

// Temporal define common interface for RoadRunner plugins.
type Temporal interface {
	GetClient() client.Client
	GetDataConverter() converter.DataConverter
	GetConfig() Config
	GetCodec() rrt.Codec
	CreateWorker(taskQueue string, options worker.Options) (worker.Worker, error)
}

// Init initiates temporal client plugin.
func (p *Plugin) Init(cfg config.Configurer, log logger.Logger) error {
	const op = errors.Op("temporal_client_plugin_init")
	if !cfg.Has(PluginName) {
		return errors.E(op, errors.Disabled)
	}

	p.dc = rrt.NewDataConverter(converter.GetDefaultDataConverter())
	err := cfg.UnmarshalKey(PluginName, &p.cfg)
	if err != nil {
		return errors.E(op, err)
	}

	p.log = log
	p.cfg.InitDefault()

	return nil
}

// GetConfig returns temporal configuration.
func (p *Plugin) GetConfig() Config {
	if p.cfg != nil {
		return *p.cfg
	}
	// empty
	return Config{}
}

// GetCodec returns communication codec.
func (p *Plugin) GetCodec() rrt.Codec {
	if p.cfg.Codec == "json" {
		return rrt.NewJSONCodec(rrt.DebugLevel(p.cfg.DebugLevel), p.log)
	}

	// production ready protocol, no debug abilities
	return rrt.NewProtoCodec()
}

// GetDataConverter returns data active data converter.
func (p *Plugin) GetDataConverter() converter.DataConverter {
	return p.dc
}

// Serve starts temporal srv.
func (p *Plugin) Serve() chan error {
	const op = errors.Op("temporal_client_plugin_serve")
	errCh := make(chan error, 1)
	worker.SetStickyWorkflowCacheSize(p.cfg.CacheSize)

	var ms tally.Scope
	var err error
	if p.cfg.Metrics != nil {
		ms, p.tallyCloser, err = newPrometheusScope(prometheus.Configuration{
			ListenAddress: p.cfg.Metrics.Address,
			TimerType:     p.cfg.Metrics.Type,
		}, p.cfg.Metrics.Prefix, p.log)
		if err != nil {
			errCh <- errors.E(op, err)
			return errCh
		}
	}

	p.client, err = client.NewClient(client.Options{
		Logger:        p.log,
		HostPort:      p.cfg.Address,
		Namespace:     p.cfg.Namespace,
		DataConverter: p.dc,
		MetricsScope:  ms,
	})

	if err != nil {
		errCh <- errors.E(op, err)
		return errCh
	}

	p.log.Debug("connected to temporal server", "address", p.cfg.Address)

	return errCh
}

// Stop stops temporal srv connection.
func (p *Plugin) Stop() error {
	if p.client != nil {
		p.client.Close()
	}

	if p.tallyCloser != nil {
		return p.tallyCloser.Close()
	}

	return nil
}

// GetClient returns active srv connection.
func (p *Plugin) GetClient() client.Client {
	return p.client
}

// CreateWorker allocates new temporal worker on an active connection.
func (p *Plugin) CreateWorker(tq string, options worker.Options) (worker.Worker, error) {
	const op = errors.Op("temporal_client_plugin_create_worker")
	if p.client == nil {
		return nil, errors.E(op, errors.Str("unable to create worker, invalid temporal client"))
	}

	if options.Identity == "" {
		if tq == "" {
			tq = client.DefaultNamespace
		}

		// ensures unique worker IDs
		options.Identity = fmt.Sprintf(
			"%d@%s@%s@%v",
			os.Getpid(),
			getHostName(),
			tq,
			atomic.AddInt32(&p.workerID, 1),
		)
	}

	return worker.New(p.client, tq, options), nil
}

// Name of the service.
func (p *Plugin) Name() string {
	return PluginName
}

func getHostName() string {
	hostName, err := os.Hostname()
	if err != nil {
		hostName = "Unknown"
	}
	return hostName
}
