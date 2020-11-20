package workflow

import (
	bindings "go.temporal.io/sdk/internalbindings"
	"sync"
)

const (
	activityType = iota + 900
	localActivityType
	timerType
	childWorkflowType
)

type (
	canceller struct {
		ids sync.Map
	}

	entry struct {
		id uint64
		tp int
	}
)

func (c *canceller) register(id uint64, target interface{}) {
	c.ids.Store(id, target)
}

func (c *canceller) discard(id uint64) {
	c.ids.Delete(id)
}

func (c *canceller) cancel(env bindings.WorkflowEnvironment, ids ...uint64) error {
	//for _, id := range ids {
	//	target, ok := c.ids.LoadAndDelete(id)
	//	if ok == false {
	//		continue
	//	}

	//switch target.(type) {
	//case *bindings.ActivityID:
	//case *TimerInfo:
	//}
	//}

	return nil
}
