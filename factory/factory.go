package factory

import (
	"github.com/temporalio/roadrunner-temporal/roadrunner"
)

type PoolOptions struct {
	NumWorkers int
	MaxJobs    int

	MaxMemory int
	// todo: controller goes here
	// todo: execution timeouts must go here
}

type WorkerFactory interface {
	NewWorker(env Env) (roadrunner.WorkerBase, error)
	NewWorkerPool(opt PoolOptions, env Env) (roadrunner.Pool, error)
}

func (wf WorkerFactory) NewWorkerPool(opt PoolOptions, env Env) (roadrunner.Pool, error) {
	return roadrunner.NewPool(
			func() (roadrunner.WorkerBase, error) {
				return appPreocvider.Swpath(env)
			}),
		opts,
	),

	// event listenters
}
