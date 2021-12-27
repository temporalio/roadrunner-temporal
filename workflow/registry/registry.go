package registry

import (
	"sync"

	bindings "github.com/spiral/sdk-go/internalbindings"
)

// IDRegistry used to gain access to child workflow ids after they become available via callback result.
type IDRegistry struct {
	sync.Mutex
	ids       sync.Map
	listeners sync.Map
}

type Listener func(w bindings.WorkflowExecution, err error)

type entry struct {
	w   bindings.WorkflowExecution
	err error
}

func (c *IDRegistry) Listen(id uint64, cl Listener) {
	c.listeners.Store(id, cl)
	val, exist := c.ids.Load(id)
	if exist {
		c.Lock()
		e := val.(entry)
		cl(e.w, e.err)
		c.Unlock()
	}
}

func (c *IDRegistry) Push(id uint64, w bindings.WorkflowExecution, err error) {
	c.ids.Store(id, entry{w: w, err: err})
	l, exist := c.listeners.Load(id)
	if exist {
		c.Lock()
		list := l.(Listener)
		list(w, err)
		c.Unlock()
	}
}
