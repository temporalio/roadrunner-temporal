package roadrunner

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrWatcherStopped = errors.New("watcher stopped")

type Stack struct {
	workers []WorkerBase
	mutex   sync.Mutex
	destroy bool
}

func NewWorkersStack() *Stack {
	return &Stack{
		workers: make([]WorkerBase, 0, 12),
	}
}

func (stack *Stack) Reset() {
	stack.mutex.Lock()
	defer stack.mutex.Unlock()

	stack.workers = nil
}

func (stack *Stack) Push(w WorkerBase) {
	stack.mutex.Lock()
	defer stack.mutex.Unlock()

	stack.workers = append(stack.workers, w)
}

func (stack *Stack) IsEmpty() bool {
	stack.mutex.Lock()
	defer stack.mutex.Unlock()

	return len(stack.workers) == 0
}

func (stack *Stack) Pop() (WorkerBase, bool) {
	stack.mutex.Lock()
	defer stack.mutex.Unlock()
	// do not release new workers
	if stack.destroy {
		return nil, true
	}

	if len(stack.workers) == 0 {
		return nil, false
	}

	w := stack.workers[len(stack.workers)-1]
	stack.workers = stack.workers[:len(stack.workers)-1]

	return w, false
}

type WorkersWatcher struct {
	workers           *Stack
	allocator         func(args ...interface{}) (*SyncWorker, error)
	initialNumWorkers int64
	actualNumWorkers  int64
	events            chan PoolEvent
}

type WorkerWatcher interface {
	// AddToWatch used to add workers to wait its state
	AddToWatch(ctx context.Context, workers []WorkerBase) error
	// GetFreeWorker provide first free worker
	GetFreeWorker(ctx context.Context) (WorkerBase, error)
	// PutWorker enqueues worker back
	PushWorker(w WorkerBase)
	// AllocateNew used to allocate new worker and put in into the WorkerWatcher
	AllocateNew(ctx context.Context) error
	// Destroy destroys the underlying workers
	Destroy(ctx context.Context)
	// WorkersList return all workers w/o removing it from internal storage
	WorkersList(ctx context.Context) []WorkerBase
}

// workerCreateFunc can be nil, but in that case, dead workers will not be replaced
func NewWorkerWatcher(allocator func(args ...interface{}) (*SyncWorker, error), numWorkers int64, events chan PoolEvent) *WorkersWatcher {
	// todo check if events not nil
	ww := &WorkersWatcher{
		workers:           NewWorkersStack(),
		allocator:         allocator,
		initialNumWorkers: numWorkers,
		actualNumWorkers:  numWorkers,
		events:            events,
	}

	return ww
}

func (ww *WorkersWatcher) AddToWatch(ctx context.Context, workers []WorkerBase) error {
	for i := 0; i < len(workers); i++ {
		sw, err := NewSyncWorker(workers[i])
		if err != nil {
			return err
		}
		ww.workers.Push(sw)
		go func(swc *WorkerBase) {
			ww.watch(ctx, swc)
			ww.wait(ctx, swc)
		}(&ww.workers.workers[i])
	}
	return nil
}

func (ww *WorkersWatcher) GetFreeWorker(ctx context.Context) (WorkerBase, error) {
	// thread safe operation
	w, stop := ww.workers.Pop()
	if stop {
		return nil, ErrWatcherStopped
	}
	// no free workers
	if w == nil {
		tt := time.NewTicker(time.Millisecond * 10)
		defer tt.Stop()
		tout := time.NewTicker(time.Second * 180)
		defer tout.Stop()
		for {
			select {
			case <-tt.C:
				w, stop = ww.workers.Pop()
				if stop {
					return nil, ErrWatcherStopped
				}
				if w == nil {
					continue
				}
				ww.actualNumWorkers--
				return w, nil
			case <-tout.C:
				return nil, errors.New("no free workers")
			}
		}
	}
	ww.actualNumWorkers--
	return w, nil
}

func (ww *WorkersWatcher) AllocateNew(ctx context.Context) error {
	ww.workers.mutex.Lock()
	sw, err := ww.allocator()
	if err != nil {
		return err
	}
	ww.addToWatch(*sw)
	ww.workers.mutex.Unlock()
	ww.PushWorker(*sw)
	return nil
}

func (ww *WorkersWatcher) addToWatch(wb WorkerBase) {
	go func() {
		ww.wait(context.Background(), &wb)
	}()
}

func (ww *WorkersWatcher) reallocate(wb *WorkerBase) error {
	sw, err := ww.allocator()
	if err != nil {
		return err
	}
	*wb = *sw
	return nil
}

// O(1) operation
func (ww *WorkersWatcher) PushWorker(w WorkerBase) {
	sw := w.(SyncWorker)
	ww.actualNumWorkers++
	ww.workers.Push(sw)
}

func (ww *WorkersWatcher) ReduceWorkersCount() {
	ww.actualNumWorkers--
}

// Destroy all underlying workers (but let them to complete the task)
func (ww *WorkersWatcher) Destroy(ctx context.Context) {
	ww.workers.mutex.Lock()
	ww.workers.destroy = true
	ww.workers.mutex.Unlock()

	tt := time.NewTicker(time.Millisecond * 100)
	for {
		select {
		case <-tt.C:
			if len(ww.workers.workers) != int(ww.actualNumWorkers) {
				continue
			}
			// unnecessary mutex, but
			// just to make sure. All workers at this moment are in the stack
			// Pop operation is blocked, push can't be done, since it's not possible to pop
			ww.workers.mutex.Lock()
			for i := 0; i < len(ww.workers.workers); i++ {
				// set state for the workers in the stack (unused at the moment)
				ww.workers.workers[i].State().Set(StateDestroyed)
			}
			ww.workers.mutex.Unlock()
			tt.Stop()
			// clear
			ww.workers.Reset()
			return
		}
	}
}

// Warning, this is O(n) operation
func (ww *WorkersWatcher) WorkersList(ctx context.Context) []WorkerBase {
	ww.workers.mutex.Lock()
	defer ww.workers.mutex.Unlock()
	return ww.workers.workers
}

func (ww *WorkersWatcher) wait(ctx context.Context, w *WorkerBase) {
	err := (*w).Wait(ctx)
	if err != nil {
		ww.events <- PoolEvent{Payload: WorkerEvent{
			Event:   EventWorkerError,
			Worker:  *w,
			Payload: err,
		}}
	}
	// If not destroyed, reallocate
	if (*w).State().Value() != StateDestroyed {
		err = ww.AllocateNew(ctx)
		if err != nil {
			ww.events <- PoolEvent{Payload: WorkerEvent{
				Event:   EventWorkerError,
				Worker:  *w,
				Payload: err,
			}}
			return
		}
	}
}

func (ww *WorkersWatcher) watch(ctx context.Context, swc *WorkerBase) {
	// todo make event to stop function
	go func() {
		select {
		case ev := <-(*swc).Events():
			ww.events <- PoolEvent{Payload: ev}
		}
	}()
}
