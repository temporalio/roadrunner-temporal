package tests

import (
	"context"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/client"
	"os"
	"testing"
	"time"
)

func Test_WorkerError_DisasterRecovery(t *testing.T) {
	s := NewTestServer()
	defer s.MustClose()

	p, err := os.FindProcess(int(s.workflows.Workers()[0].Pid()))
	assert.NoError(t, err)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue:           "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	time.Sleep(time.Millisecond * 750)

	// must fully recover with new worker
	assert.NoError(t, p.Kill())

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)
}
