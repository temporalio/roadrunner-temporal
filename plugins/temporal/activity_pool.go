package temporal

import (
	"context"
	"github.com/spiral/roadrunner/v2"
	"github.com/spiral/roadrunner/v2/plugins/factory"
	"log"
)

type ActivityPool struct {
	temporal *Provider
	wFactory factory.WorkerFactory
	pool     roadrunner.Pool
}

// logger dep also
func (a *ActivityPool) Init(
	temporal *Provider,
	wFactory factory.WorkerFactory,
) error {
	a.temporal = temporal
	a.wFactory = wFactory
	return nil
}

func (a *ActivityPool) Serve() chan error {
	errCh := make(chan error)
	if a.temporal.config.Activities != nil {
		go a.runWorkers(errCh)
	} else {
		log.Println("disabled", a.temporal.serviceClient)
	}

	return errCh
}

func (a *ActivityPool) runWorkers(errCh chan error) {
	pool, err := a.wFactory.NewWorkerPool(context.Background(), a.temporal.config.Activities, map[string]string{
		"RR_ACTIVITY_POOL": "true", // todo: temporal specific
	})

	if err != nil {
		errCh <- err
		return
	}

	a.pool = pool

	r, err := pool.Exec(context.Background(), roadrunner.Payload{Body: []byte("hello"), Context: []byte("jello")})
	if err != nil {
		errCh <- err
		return
	}

	log.Print(r)

}

func (a *ActivityPool) Stop() error {
	if a.pool != nil {
		a.pool.Destroy(context.Background())
	}

	return nil
}
