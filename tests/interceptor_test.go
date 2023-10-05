package tests

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/client"
)

func Test_CustomInterceptor(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServerWithInterceptor(t, stopCh, wg)

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleWorkflow",
		"test-input",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "TEST-INPUT", result)

	_, err = os.Stat("./interceptor_test")
	assert.NoError(t, err)

	we, err := s.Client.DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	assert.Equal(t, "Completed", we.WorkflowExecutionInfo.Status.String())
	stopCh <- struct{}{}
	wg.Wait()

	t.Cleanup(func() {
		_ = os.Remove("interceptor_test")
	})
}
