package resetter

import (
	"github.com/spiral/errors"
)

const PluginName = "resetter"

type ResetTarget interface {
	Name() string
	Reset() error
}

type Plugin struct {
	registry map[string]ResetTarget
}

func (p *Plugin) Init() error {
	p.registry = make(map[string]ResetTarget)
	return nil
}

// Reset named service.
func (p *Plugin) Reset(name string) error {
	svc, ok := p.registry[name]
	if !ok {
		return errors.E("no such service", errors.Str(name))
	}

	return svc.Reset()
}

// RegisterTarget resettable service.
func (p *Plugin) RegisterTarget(r ResetTarget) error {
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
