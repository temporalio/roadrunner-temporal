package workflow

import (
	bindings "go.temporal.io/sdk/internalbindings"
	"sync"
)

type (
	canceller struct {
		ids sync.Map
	}

	cancellable func() error
)

func (c *canceller) register(id uint64, cancel cancellable) {
	c.ids.Store(id, cancel)
}

func (c *canceller) discard(id uint64) {
	c.ids.Delete(id)
}

func (c *canceller) cancel(env bindings.WorkflowEnvironment, ids ...uint64) error {
	var err error
	for _, id := range ids {
		cancel, ok := c.ids.LoadAndDelete(id)
		if ok == false {
			continue
		}

		err = cancel.(cancellable)()
		if err != nil {
			return err
		}
	}

	return nil
}
