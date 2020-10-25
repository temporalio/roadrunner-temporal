package temporal

import (
	"github.com/spiral/roadrunner/v2/plugins/config"
	rrt "github.com/temporalio/roadrunner-temporal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// inherit roadrunner.rpc.Plugin interface
type Server struct {
	// Temporal config from .rr.yaml
	config Config

	// Temporal connection
	client client.Client
}

// logger dep also
func (p *Server) Init(config config.Provider) error {
	return config.UnmarshalKey(ServiceName, &p.config)
}

func (p *Server) GetConfig() Config {
	return p.config
}

func (p *Server) Serve() chan error {
	errCh := make(chan error, 1)
	var err error

	p.client, err = client.NewClient(client.Options{
		HostPort:      p.config.Address,
		Namespace:     p.config.Namespace,
		DataConverter: rrt.NewRRDataConverter(),
	})
	if err != nil {
		errCh <- err
	}

	return errCh
}

func (p *Server) Stop() error {
	p.client.Close()
	return nil
}

func (p *Server) GetClient() (client.Client, error) {
	return p.client, nil
}

func (p *Server) CreateWorker(tq string, options worker.Options) (worker.Worker, error) {
	w := worker.New(p.client, tq, options)
	return w, nil
}

func (p *Server) Name() string {
	return ServiceName
}

func (p *Server) RPCService() (interface{}, error) {
	c, err := p.GetClient()
	if err != nil {
		return nil, err
	}

	return &Rpc{
		client: c,
	}, nil
}
