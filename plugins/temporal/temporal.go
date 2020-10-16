package temporal

import (
	"github.com/spiral/roadrunner/v2/plugins/config"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

const temporalSection = "temporal"

type Temporal interface {
	GetClient() (client.Client, error) // our data converter here
	CreateWorker(taskQueue string, options worker.Options) (worker.Worker, error)
}

type Plugin struct {
	configProvider config.Provider
	config         Config
	serviceClient  client.Client
}

// logger dep also
func (p *Plugin) Init(provider config.Provider) error {
	p.configProvider = provider
	return nil
}

func (p *Plugin) Configure() error {
	c := &Config{}
	err := p.configProvider.UnmarshalKey(temporalSection, c)
	if err != nil {
		return err
	}
	return nil
}

func (p *Plugin) Close() error {
	return nil
}

func (p *Plugin) Serve() chan error {
	errCh := make(chan error)
	var err error
	p.serviceClient, err = client.NewClient(client.Options{
		HostPort: p.config.Address,
	})
	if err != nil {
		errCh <- err
	}
	return errCh
}

func (p *Plugin) Stop() error {
	p.serviceClient.Close()
	return nil
}

func (p *Plugin) GetClient() (client.Client, error) {
	return p.serviceClient, nil
}

func (p *Plugin) CreateWorker(tq string, options worker.Options) (worker.Worker, error) {
	w := worker.New(p.serviceClient, tq, options)
	return w, nil
}
