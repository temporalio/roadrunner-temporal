package temporal

import (
	"fmt"
	"os"
	"sync/atomic"

	"github.com/spiral/errors"
	poolImpl "github.com/spiral/roadrunner/v2/pkg/pool"
	"github.com/spiral/roadrunner/v2/plugins/config"
	"github.com/spiral/roadrunner/v2/plugins/logger"
	rrt "github.com/temporalio/roadrunner-temporal/protocol"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/worker"
)

const PluginName = "temporal"

type (
	Temporal interface {
		GetClient() (client.Client, error)
		GetDataConverter() converter.DataConverter
		GetConfig() Config
		CreateWorker(taskQueue string, options worker.Options) (worker.Worker, error)
	}

	Config struct {
		Address    string
		Namespace  string
		Activities *poolImpl.Config
	}

	Plugin struct {
		workerID int32
		cfg      Config
		dc       converter.DataConverter
		log      logger.Logger
		client   client.Client
	}
)

// logger dep also
func (srv *Plugin) Init(cfg config.Configurer, log logger.Logger) error {
	srv.log = log
	srv.dc = rrt.NewDataConverter(converter.GetDefaultDataConverter())

	return cfg.UnmarshalKey(PluginName, &srv.cfg)
}

// GetConfig returns temporal configuration.
func (srv *Plugin) GetConfig() Config {
	return srv.cfg
}

// GetDataConverter returns data active data converter.
func (srv *Plugin) GetDataConverter() converter.DataConverter {
	return srv.dc
}

// Serve starts temporal srv.
func (srv *Plugin) Serve() chan error {
	errCh := make(chan error, 1)
	var err error

	srv.client, err = client.NewClient(client.Options{
		Logger:        srv.log,
		HostPort:      srv.cfg.Address,
		Namespace:     srv.cfg.Namespace,
		DataConverter: srv.dc,
	})

	srv.log.Debug("Connected to temporal server", "Plugin", srv.cfg.Address)

	if err != nil {
		errCh <- errors.E(errors.Op("srv connect"), err)
	}

	return errCh
}

// Stop stops temporal srv connection.
func (srv *Plugin) Stop() error {
	if srv.client != nil {
		srv.client.Close()
	}

	return nil
}

// GetClient returns active srv connection.
func (srv *Plugin) GetClient() (client.Client, error) {
	return srv.client, nil
}

// CreateWorker allocates new temporal worker on an active connection.
func (srv *Plugin) CreateWorker(tq string, options worker.Options) (worker.Worker, error) {
	if srv.client == nil {
		return nil, errors.E("unable to create worker, invalid temporal srv")
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
			atomic.AddInt32(&srv.workerID, 1),
		)
	}

	return worker.New(srv.client, tq, options), nil
}

// Name of the service.
func (srv *Plugin) Name() string {
	return PluginName
}

// RPCService returns associated rpc service.
func (srv *Plugin) RPC() interface{} {
	return &rpc{srv: srv}
}

func getHostName() string {
	hostName, err := os.Hostname()
	if err != nil {
		hostName = "Unknown"
	}
	return hostName
}
