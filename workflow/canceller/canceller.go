package canceller

import (
	"sync"
)

type Cancellable func() error

type Canceller struct {
	ids sync.Map
}

func (c *Canceller) Register(id uint64, cancel Cancellable) {
	c.ids.Store(id, cancel)
}

func (c *Canceller) Discard(id uint64) {
	c.ids.Delete(id)
}

func (c *Canceller) Cancel(ids ...uint64) error {
	var err error
	for _, id := range ids {
		cancel, ok := c.ids.LoadAndDelete(id)
		if !ok {
			continue
		}

		err = cancel.(Cancellable)()
		if err != nil {
			return err
		}
	}

	return nil
}
