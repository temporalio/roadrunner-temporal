package roadrunner

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const MB = 1024 * 1024

type Supervisor interface {
	Attach(pool Pool)
	StartWatching() error
	StopWatching() error
	Detach()
}

type staticPoolSupervisor struct {
	// maxWorkerMemory in MB
	maxWorkerMemory uint64
	// maxWorkerTTL in seconds
	maxWorkerTTL uint64
	// maxWorkerIdle in seconds
	maxWorkerIdle uint64

	pool Pool
}

func NewStaticPoolSupervisor(mwm uint64, maxTtl uint64, maxIdle uint64) Supervisor {
	return &staticPoolSupervisor{
		maxWorkerMemory: mwm,
		maxWorkerTTL:    maxTtl,
		maxWorkerIdle:   maxIdle,
	}
}

func (sps *staticPoolSupervisor) Attach(pool Pool) {
	sps.pool = pool
}

func (sps *staticPoolSupervisor) StartWatching() error {
	return nil
}

func (sps *staticPoolSupervisor) StopWatching() error {
	return nil
}

func (sps *staticPoolSupervisor) Detach() {

}

func (sps *staticPoolSupervisor) control(p Pool) error {
	if sps.pool == nil {
		return errors.New("pool should be attached")
	}
	now := time.Now()
	ctx := context.TODO()

	// THIS IS A COPY OF WORKERS
	workers := p.Workers(ctx)

	for i := 0; i < len(workers); i++ {
		if workers[i].State().Value() == StateInvalid {
			continue
		}

		s, err := WorkerProcessState(workers[i])
		if err != nil {
			panic(err)
			// push to pool events??
		}

		if sps.maxWorkerTTL != 0 && now.Sub(workers[i].Created()).Seconds() >= float64(sps.maxWorkerTTL) {
			err = p.RemoveWorker(ctx, workers[i])
			if err != nil {
				panic(err)
			}

			// after remove worker we should exclude it from further analysis
			workers = append(workers[:i], workers[i+1:]...)
		}

		if sps.maxWorkerMemory != 0 && s.MemoryUsage >= sps.maxWorkerMemory*MB {
			// TODO events
			p.Events() <- PoolEvent{Payload: fmt.Errorf("max allowed memory reached (%vMB)", sps.maxWorkerMemory)}
			err = p.RemoveWorker(ctx, workers[i])
			if err != nil {
				// TODO
				panic(err)
			}
			continue
		}

		// firs we check maxWorker idle
		if sps.maxWorkerIdle != 0 {
			// then check for the worker state
			if workers[i].State().Value() != StateReady {
				continue
			}
			/*
				Calculate idle time
				If worker in the StateReady, we read it LastUsed timestamp as UnixNano uint64
				2. For example maxWorkerIdle is equal to 5sec, then, if (time.Now - LastUsed) > maxWorkerIdle
				we are guessing that worker overlap idle time and has to be killed
			*/
			// get last used unix nano
			lu := workers[i].State().LastUsed()
			// convert last used to unixNano and sub time.now
			res := int64(lu) - now.UnixNano()
			// maxWorkerIdle more than diff between now and last used
			if int64(sps.maxWorkerIdle)-res <= 0 {
				p.Events() <- PoolEvent{Payload: fmt.Errorf("max allowed worker idle time elapsed. actual idle time: %v, max idle time: %v", sps.maxWorkerIdle, res)}
				err = p.RemoveWorker(ctx, workers[i])
				if err != nil {
					// TODO
					panic(err)
				}
			}
		}

	}

	return nil
}
