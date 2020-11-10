package resetter

import "github.com/spiral/errors"

const ServiceName = "resetter"

type NamedResetter interface {
	Name() string
	Reset() error
}

type Plugin struct {
	registry map[string]NamedResetter
}

func (p *Plugin) Init() error {
	p.registry = make(map[string]NamedResetter)
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

// Register resettable service.
func (p *Plugin) Register(r NamedResetter) error {
	p.registry[r.Name()] = r
	return nil
}

// Collects declares services to be collected.
func (p *Plugin) Collects() []interface{} {
	return []interface{}{
		p.Register,
	}
}

// Name of the service.
func (p *Plugin) Name() string {
	return ServiceName
}

// RPCService returns associated rpc service.
func (p *Plugin) RPCService() (interface{}, error) {
	return &rpc{srv: p}, nil
}
