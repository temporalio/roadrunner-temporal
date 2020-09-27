package roadrunner

import (
	"fmt"
	"runtime"
	"time"
)

const (
	// EventWorkerConstruct thrown when new worker is spawned.
	EventWorkerConstruct = iota + 100

	// EventWorkerDestruct thrown after worker destruction.
	EventWorkerDestruct

	// EventWorkerKill thrown after worker is being forcefully killed.
	EventWorkerKill

	// EventWorkerError thrown any worker related even happen (passed with WorkerError)
	EventWorkerEvent

	// EventWorkerDead thrown when worker stops worker for any reason.
	EventWorkerDead

	// EventPoolError caused on pool wide errors
	EventPoolError
)

// todo: merge with pool options

// Config defines basic behaviour of worker creation and handling process.
type Config struct {
	// NumWorkers defines how many sub-processes can be run at once. This value
	// might be doubled by Swapper while hot-swap.
	NumWorkers int64

	// MaxJobs defines how many executions is allowed for the worker until
	// it's destruction. set 1 to create new process for each new task, 0 to let
	// worker handle as many tasks as it can.
	MaxJobs int64

	// AllocateTimeout defines for how long pool will be waiting for a worker to
	// be freed to handle the task.
	AllocateTimeout time.Duration

	// DestroyTimeout defines for how long pool should be waiting for worker to
	// properly stop, if timeout reached worker will be killed.
	DestroyTimeout time.Duration
}

// InitDefaults allows to init blank config with pre-defined set of default values.
func (cfg *Config) InitDefaults() error {
	cfg.AllocateTimeout = time.Minute
	cfg.DestroyTimeout = time.Minute
	cfg.NumWorkers = int64(runtime.NumCPU())

	return nil
}

// Valid returns error if config not valid.
func (cfg *Config) Valid() error {
	if cfg.NumWorkers == 0 {
		return fmt.Errorf("pool.NumWorkers must be set")
	}

	if cfg.AllocateTimeout == 0 {
		return fmt.Errorf("pool.AllocateTimeout must be set")
	}

	if cfg.DestroyTimeout == 0 {
		return fmt.Errorf("pool.DestroyTimeout must be set")
	}

	return nil
}

// Pool managed set of inner worker processes.
type Pool interface {
	Events() chan PoolEvent

	// Exec one task with given payload and context, returns result or error.
	Exec(rqs Payload) (rsp Payload, err error)

	// Workers returns worker list associated with the pool.
	Workers() (workers []Worker)

	// Remove forces pool to remove specific worker. Return true is this is first remove request on given worker.
	Remove(w Worker, err error) bool

	// Destroy all underlying workers (but let them to complete the task).
	Destroy()
}
