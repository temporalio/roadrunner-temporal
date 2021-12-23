package internal

import (
	"context"

	"github.com/spiral/sdk-go/worker"
)

type Activity interface {
	GetActivityContext(taskToken []byte) (context.Context, error)
	Init() ([]worker.Worker, error)
	ActivityNames() []string
}

type Workflow interface {
	WorkflowNames() []string
	Init() ([]worker.Worker, error)
}
