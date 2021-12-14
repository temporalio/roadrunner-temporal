package roadrunner_temporal

import (
	"sync"
	"time"

	"github.com/spiral/errors"
	"github.com/spiral/roadrunner-plugins/v2/config"
	"github.com/spiral/roadrunner-plugins/v2/logger"
	"github.com/spiral/roadrunner-plugins/v2/server"
	temporalClient "github.com/spiral/sdk-go/client"
	"github.com/spiral/sdk-go/converter"
)

// PluginName defines public service name.
const (
	PluginName = "temporal"

	// RRMode sets as RR_MODE env variable to let worker know about the mode to run.
	RRMode = "temporal"
)

type Plugin struct {
	sync.RWMutex

	log logger.Logger

	client        *temporalClient.Client
	dataConverter converter.DataConverter

	graceTimeout time.Duration
}

func (p *Plugin) Init(cfg config.Configurer, server server.Server, log logger.Logger) error {
	const op = errors.Op("temporal_plugin_init")

	if !cfg.Has(PluginName) {
		return errors.E(op, errors.Disabled)
	}

	p.log = log

	return nil
}

func (p *Plugin) Serve() chan error {
	errCh := make(chan error, 1)
	return errCh
}

func (p *Plugin) Stop() error {
	return nil
}
