package informer

import (
	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2"
)

const PluginName = "informer"

type InformTarget interface {
	Name() string
	Workers() []roadrunner.WorkerBase
}

type Plugin struct {
	registry map[string]InformTarget
}

func (p *Plugin) Init() error {
	p.registry = make(map[string]InformTarget)
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
func (p *Plugin) RegisterTarget(r InformTarget) error {
	p.registry[r.Name()] = r
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
func (p *Plugin) RPCService() (interface{}, error) {
	return &rpc{srv: p}, nil
}
