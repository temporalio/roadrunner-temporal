package registry

import (
	"sync"

	bindings "go.temporal.io/sdk/internalbindings"
)

// Registry stores at most one (value, err) entry per uint64 ID and at most one
// listener per ID. Push delivers to a registered listener (if any); Listen
// fires immediately if a Push has already happened for that ID, otherwise it
// waits for the next Push.
type Registry[T any] struct {
	sync.Mutex
	ids       sync.Map
	listeners sync.Map
}

// ListenerFunc is invoked once per ID with the delivered (value, err) pair.
type ListenerFunc[T any] func(value T, err error)

type entry[T any] struct {
	value T
	err   error
}

func (c *Registry[T]) Listen(id uint64, cl ListenerFunc[T]) {
	c.listeners.Store(id, cl)
	val, exist := c.ids.Load(id)
	if exist {
		c.Lock()
		e := val.(entry[T])
		cl(e.value, e.err)
		c.Unlock()
	}
}

func (c *Registry[T]) Push(id uint64, value T, err error) {
	c.ids.Store(id, entry[T]{value: value, err: err})
	l, exist := c.listeners.Load(id)
	if exist {
		c.Lock()
		list := l.(ListenerFunc[T])
		list(value, err)
		c.Unlock()
	}
}

// Discard drops any stored entry and listener for id. Idempotent; safe to call
// when nothing is registered. Owners of an ID call this once they know no
// further Push or Listen for that ID is meaningful — e.g. when the wrapping
// operation has fully completed — to bound memory growth.
func (c *Registry[T]) Discard(id uint64) {
	c.ids.Delete(id)
	c.listeners.Delete(id)
}

// IDRegistry used to gain access to child workflow ids after they become available via callback result.
type IDRegistry = Registry[bindings.WorkflowExecution]

// Listener is the listener type for IDRegistry.
type Listener = ListenerFunc[bindings.WorkflowExecution]
