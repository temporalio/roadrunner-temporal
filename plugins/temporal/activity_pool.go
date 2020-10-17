package temporal

import (
	"context"
	"github.com/spiral/roadrunner/v2"
	"go.temporal.io/sdk/worker"
)

// ActivityPool manages set of RR and Temporal activity workers and their cancellation contexts.
type ActivityPool struct {
	workerPool      roadrunner.Pool
	temporalWorkers []worker.Worker
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the pool.
func (p *ActivityPool) InitTemporal(temporal Temporal) error {
	//r, err := pool.Exec(context.Background(), roadrunner.Payload{Body: []byte("hello"), Context: []byte("jello")})
	//if err != nil {
	//	errCh <- err
	//	return
	//}
}

func (p *ActivityPool) Serve(errChan chan error) {

}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the pool.
func (p *ActivityPool) Destroy(ctx context.Context) {

}
