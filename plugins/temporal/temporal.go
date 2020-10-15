package temporal

import (
	"github.com/spiral/roadrunner/v2/plugins/config"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

type Temporal interface {
	GetClient() (client.Client, error) // our data converter here
	CreateWorker(taskQueue string, options worker.Options) (worker.Worker, error)
}

type Trm struct {
}

// logger dep also
func (t *Trm) Init(provider config.Provider) error {
	// section temporal
	return nil
}
