package tests

import (
	"context"
	"sync"
	"testing"
	"tests/helpers"

	"github.com/stretchr/testify/assert"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
)

func Test_CustomDataConverter(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServerWithDataConverter(t, stopCh, wg)

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

	we, err := s.Client.DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	assert.Equal(t, "Completed", we.WorkflowExecutionInfo.Status.String())

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		if event.EventType != enums.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED {
			return false
		}
		attrs := event.GetWorkflowExecutionStartedEventAttributes()
		if attrs == nil || attrs.GetInput() == nil {
			return false
		}
		payloads := attrs.GetInput().GetPayloads()
		if len(payloads) == 0 {
			return false
		}
		encoding, ok := payloads[0].Metadata["encoding"]
		if !ok {
			return false
		}
		assert.Equal(t, "json/test-custom", string(encoding))
		return true
	})

	stopCh <- struct{}{}
	wg.Wait()
}
