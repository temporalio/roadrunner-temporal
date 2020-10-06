package roadrunner

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/pkg/errors"
)

const (
	// StopRequest can be sent by worker to indicate that restart is required.
	StopRequest = "{\"stop\":true}"
)

// StaticPool controls worker creation, destruction and task routing. Pool uses fixed amount of workers.
type StaticPool struct {
	// pool behaviour
	cfg Config

	// worker command creator
	cmd func() *exec.Cmd

	// creates and connects to workers
	factory Factory

	// protects state of worker list, does not affect allocation
	muw sync.RWMutex

	ww *WorkersWatcher

	events chan PoolEvent
}
type PoolEvent struct {
	Payload interface{}
}

// NewPool creates new worker pool and task multiplexer. StaticPool will initiate with one worker.
// supervisor Supervisor, todo: think about it
// workers func() (WorkerBase, error),
func NewPool(cmd func() *exec.Cmd, factory Factory, cfg Config) (Pool, error) {
	if err := cfg.Valid(); err != nil {
		return nil, errors.Wrap(err, "config")
	}

	ctx := context.Background()
	p := &StaticPool{
		cfg:     cfg,
		cmd:     cmd,
		factory: factory,
		events:  make(chan PoolEvent),
	}

	p.ww = NewWorkerWatcher(func(args ...interface{}) (*SyncWorker, error) {
		w, err := p.factory.SpawnWorker(ctx, p.cmd())
		if err != nil {
			return nil, err
		}

		sw, err := NewSyncWorker(w)
		if err != nil {
			return nil, err
		}
		return &sw, nil
	}, p.cfg.NumWorkers, p.events)

	workers, err := p.allocateWorkers(ctx, p.cfg.NumWorkers)
	if err != nil {
		return nil, err
	}

	// put workers in the pool
	err = p.ww.AddToWatch(ctx, workers)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// allocate required number of workers
func (p *StaticPool) allocateWorkers(ctx context.Context, numWorkers int64) ([]WorkerBase, error) {
	var workers []WorkerBase

	// constant number of workers simplify logic
	for i := int64(0); i < numWorkers; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), p.cfg.AllocateTimeout)
		w, err := p.factory.SpawnWorker(ctx, p.cmd())
		if err != nil {
			cancel()
			return nil, err
		}
		cancel()
		workers = append(workers, w)
	}
	return workers, nil
}

// Config returns associated pool configuration. Immutable.
func (p *StaticPool) Config() Config {
	return p.cfg
}

// Workers returns worker list associated with the pool.
func (p *StaticPool) Workers(ctx context.Context) (workers []WorkerBase) {
	return p.ww.WorkersList(ctx)
}

// Exec one task with given payload and context, returns result or error.
func (p *StaticPool) Exec(ctx context.Context, rqs Payload) (Payload, error) {
	getWorkerCtx, cancel := context.WithTimeout(context.TODO(), p.cfg.AllocateTimeout)
	defer cancel()
	w, err := p.ww.GetFreeWorker(getWorkerCtx)
	if err != nil && errors.Is(err, ErrWatcherStopped) {
		return EmptyPayload, ErrWatcherStopped
	} else if err != nil {
		return EmptyPayload, err
	}

	sw := w.(SyncWorker)

	execCtx, cancel2 := context.WithTimeout(context.TODO(), p.cfg.ExecTTL)
	defer cancel2()

	rsp, err := sw.Exec(execCtx, rqs)
	if err != nil {
		errJ := p.checkMaxJobs(ctx, w)
		if errJ != nil {
			return EmptyPayload, fmt.Errorf("%v, %v", err, errJ)
		}
		// soft job errors are allowed
		if _, jobError := err.(TaskError); jobError {
			p.ww.PushWorker(w)
			return EmptyPayload, err
		}

		sw.State().Set(StateInvalid)
		errS := w.Stop(ctx)
		if errS != nil {
			return EmptyPayload, fmt.Errorf("%v, %v", err, errS)
		}

		return EmptyPayload, err
	}

	// worker want's to be terminated
	if rsp.Body == nil && rsp.Context != nil && string(rsp.Context) == StopRequest {
		w.State().Set(StateInvalid)
		err = w.Stop(ctx)
		if err != nil {
			panic(err)
		}

		return p.Exec(ctx, rqs)
	}

	if p.cfg.MaxJobs != 0 && w.State().NumExecs() >= p.cfg.MaxJobs {
		err = p.ww.AllocateNew(ctx)
		if err != nil {
			return EmptyPayload, err
		}
	} else {
		p.ww.PushWorker(w)
	}
	return rsp, nil
}

func (p *StaticPool) checkMaxJobs(ctx context.Context, w WorkerBase) error {
	if p.cfg.MaxJobs != 0 && w.State().NumExecs() >= p.cfg.MaxJobs {
		err := p.ww.AllocateNew(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// Destroy all underlying workers (but let them to complete the task).
func (p *StaticPool) Destroy(ctx context.Context) {
	p.ww.Destroy(ctx)
}

func (p *StaticPool) Events() chan PoolEvent {
	return p.events
}
