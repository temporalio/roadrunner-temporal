package factory

import "github.com/temporalio/roadrunner-temporal/roadrunner"

type PoolOptions struct {
	NumWorkers int
	MaxJobs    int

	MaxMemory int
	// todo: controller goes here
	// todo: execution timeouts must go here
}

type WorkerFactory interface {
	NewWorker(env Env) (*roadrunner.worker, error)
	NewAsyncWorker(env Env) (*roadrunner.AsyncWorker, error)
	NewWorkerPool(opt PoolOptions, env Env) (roadrunner.Pool, error)
}
