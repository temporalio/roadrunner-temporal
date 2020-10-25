package temporal

import (
	"github.com/spiral/roadrunner/v2"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

const ServiceName = "temporal"

type Config struct {
	Address    string
	Namespace  string
	Activities *roadrunner.Config
}

type Temporal interface {
	GetClient() (client.Client, error)
	GetConfig() Config
	CreateWorker(taskQueue string, options worker.Options) (worker.Worker, error)
}
