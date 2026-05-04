package registry

import (
	"sync"

	bindings "go.temporal.io/sdk/internalbindings"
)

// Registry stores at most one (value, err) entry per uint64 ID and at most one
// listener per ID. Push delivers to a registered listener (if any); Listen
// fires immediately if a Push has already happened for that ID, otherwise it
// waits for the next Push.
//
// Last-write-wins for both entries and listeners. The zero value is usable.
type Registry[T any] struct {
	sync.Mutex
	entries   sync.Map
	listeners sync.Map
}

// ListenerFunc is invoked once per (id, value, err) triple delivered.
type ListenerFunc[T any] func(value T, err error)

type entry[T any] struct {
	value T
	err   error
}

func (r *Registry[T]) Listen(id uint64, cl ListenerFunc[T]) {
	r.listeners.Store(id, cl)
	val, exist := r.entries.Load(id)
	if exist {
		r.Lock()
		e := val.(entry[T])
		cl(e.value, e.err)
		r.Unlock()
	}
}

func (r *Registry[T]) Push(id uint64, value T, err error) {
	r.entries.Store(id, entry[T]{value: value, err: err})
	l, exist := r.listeners.Load(id)
	if exist {
		r.Lock()
		list := l.(ListenerFunc[T])
		list(value, err)
		r.Unlock()
	}
}

// IDRegistry used to gain access to child workflow ids after they become available via callback result.
type IDRegistry = Registry[bindings.WorkflowExecution]

// Listener is the listener type for IDRegistry.
type Listener = ListenerFunc[bindings.WorkflowExecution]
