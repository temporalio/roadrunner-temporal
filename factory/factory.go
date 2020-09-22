package factory

type PoolOptions struct {
	NumWorkers int
	MaxJobs    int

	MaxMemory int

	// todo: execution timeouts must go here
}

type WorkerFactory interface {
	NewWorker(env Env) (*rr.Worker, error)
	NewAsyncWorker(env Env) (*rr.AsyncWorker, error)
	NewWorkerPool(opt PoolOptions, env Env) (rr.Pool, error)
}
