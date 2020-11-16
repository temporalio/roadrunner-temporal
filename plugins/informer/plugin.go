package informer

import (
	"github.com/spiral/endure"
	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2"
)

const PluginName = "informer"

type Informer interface {
	Workers() []roadrunner.WorkerBase
}

type Plugin struct {
	registry map[string]Informer
}

func (p *Plugin) Init() error {
	p.registry = make(map[string]Informer)
	return nil
}

// Reset named service.
func (p *Plugin) Workers(name string) ([]roadrunner.WorkerBase, error) {
	svc, ok := p.registry[name]
	if !ok {
		return nil, errors.E("no such service", errors.Str(name))
	}

	return svc.Workers(), nil
}

// RegisterTarget resettable service.
func (p *Plugin) RegisterTarget(name endure.Named, r Informer) error {
	p.registry[name.Name()] = r
	return nil
}

// Collects declares services to be collected.
func (p *Plugin) Collects() []interface{} {
	return []interface{}{
		p.RegisterTarget,
	}
}

// Name of the service.
func (p *Plugin) Name() string {
	return PluginName
}

// RPCService returns associated rpc service.
func (p *Plugin) RPC() interface{} {
	return &rpc{srv: p}
}
