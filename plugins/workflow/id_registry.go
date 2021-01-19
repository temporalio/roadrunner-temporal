package workflow

import (
	bindings "go.temporal.io/sdk/internalbindings"
	"sync"
)

type (
	// used to gain access to child workflow ids after they become available via callback result.
	idRegistry struct {
		mu        sync.Mutex
		ids       map[uint64]entry
		listeners map[uint64]listener
	}

	listener func(w bindings.WorkflowExecution, err error)

	entry struct {
		w   bindings.WorkflowExecution
		err error
	}
)

func newIDRegistry() *idRegistry {
	return &idRegistry{
		ids:       map[uint64]entry{},
		listeners: map[uint64]listener{},
	}
}

func (c *idRegistry) listen(id uint64, cl listener) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.listeners[id] = cl

	if e, ok := c.ids[id]; ok {
		cl(e.w, e.err)
	}
}

func (c *idRegistry) push(id uint64, w bindings.WorkflowExecution, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e := entry{w: w, err: err}
	c.ids[id] = e

	if l, ok := c.listeners[id]; ok {
		l(e.w, e.err)
	}
}
