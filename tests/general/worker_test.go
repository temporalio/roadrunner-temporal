package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	"tests/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

func Test_ResetWorkerWorkflow(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	// Create workflow options with 12-second execution timeout
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue:                "default",
		WorkflowExecutionTimeout: 12 * time.Second,
	}

	// Create client with 1-second timeout for operations
	_, clientCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer clientCancel()

	// Start the workflow with a 10-second timer parameter
	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		workflowOptions,
		"ResetWorkerWorkflow",
		10, // 10-second timer parameter
	)
	require.NoError(t, err)

	// Allow workflow to start
	time.Sleep(500 * time.Millisecond)

	// Query the workflow to kill the worker - this should timeout
	queryCtx, queryCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer queryCancel()

	_, err = s.Client.QueryWorkflow(queryCtx, w.GetID(), w.GetRunID(), "die", nil)

	// Should fail with a timeout since the worker dies during query execution
	assert.Error(t, err)

	// Cancel the workflow
	err = s.Client.CancelWorkflow(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	// Wait for workflow to complete and verify it's canceled
	resultCtx, resultCancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer resultCancel()

	var result interface{}
	err = w.Get(resultCtx, &result)

	// Should fail with a canceled error
	assert.Error(t, err)

	// Verify the workflow was actually canceled
	we, err := s.Client.DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
	require.NoError(t, err)
	assert.Equal(t, enums.WORKFLOW_EXECUTION_STATUS_CANCELED, we.WorkflowExecutionInfo.Status)

	stopCh <- struct{}{}
	wg.Wait()
}
