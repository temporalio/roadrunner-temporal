package workflow

import (
	"sync"

	bindings "go.temporal.io/sdk/internalbindings"
)

// used to gain access to child workflow ids after they become available via callback result.
type idRegistry struct {
	sync.Mutex
	ids       sync.Map
	listeners sync.Map
}

type listener func(w bindings.WorkflowExecution, err error)

type entry struct {
	w   bindings.WorkflowExecution
	err error
}

func newIDRegistry() *idRegistry {
	return &idRegistry{}
}

func (c *idRegistry) listen(id uint64, cl listener) {
	c.listeners.Store(id, cl)
	val, exist := c.ids.Load(id)
	if exist {
		c.Lock()
		e := val.(entry)
		cl(e.w, e.err)
		c.Unlock()
	}
}

func (c *idRegistry) push(id uint64, w bindings.WorkflowExecution, err error) {
	c.ids.Store(id, entry{w: w, err: err})
	l, exist := c.listeners.Load(id)
	if exist {
		c.Lock()
		list := l.(listener)
		list(w, err)
		c.Unlock()
	}
}
