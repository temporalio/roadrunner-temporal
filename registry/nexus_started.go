package registry

import "sync"

// NexusStartedRegistry mirrors IDRegistry but for the start phase of caller-side
// Nexus operations. The Temporal SDK's started callback delivers
// `(token string, err error)` — token != "" for async, "" for sync. PHP's
// GetNexusOperationStarted{ID} listens here; the listener fires either when
// Push has already been called or as soon as Push happens, whichever comes
// first, exactly like IDRegistry for child workflows.
type NexusStartedRegistry struct {
	sync.Mutex
	entries   sync.Map
	listeners sync.Map
}

type NexusStartedListener func(token string, err error)

type nexusStartedEntry struct {
	token string
	err   error
}

func (c *NexusStartedRegistry) Listen(id uint64, cl NexusStartedListener) {
	c.listeners.Store(id, cl)
	val, exist := c.entries.Load(id)
	if exist {
		c.Lock()
		e := val.(nexusStartedEntry)
		cl(e.token, e.err)
		c.Unlock()
	}
}

func (c *NexusStartedRegistry) Push(id uint64, token string, err error) {
	c.entries.Store(id, nexusStartedEntry{token: token, err: err})
	l, exist := c.listeners.Load(id)
	if exist {
		c.Lock()
		list := l.(NexusStartedListener)
		list(token, err)
		c.Unlock()
	}
}
