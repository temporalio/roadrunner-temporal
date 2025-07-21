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

	// Create workflow options with 20-second execution timeout
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue:                "default",
		WorkflowExecutionTimeout: 20 * time.Second,
	}

	// Start the workflow with a 15-second timer parameter
	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		workflowOptions,
		"ResetWorkerWorkflow",
		15, // 15-second timer parameter
	)
	require.NoError(t, err)

	// Allow workflow to start
	time.Sleep(time.Second)

	// Query the workflow to kill the worker - this should timeout
	ctx, cancel := context.WithTimeout(context.Background(), 4 * time.Second)
	_, err = s.Client.QueryWorkflow(ctx, w.GetID(), w.GetRunID(), "die", 1)
	cancel()

	// Should fail with a timeout since the worker dies during query execution
	require.Error(t, err)
	
	// Cancel the workflow
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	err = s.Client.CancelWorkflow(ctx, w.GetID(), w.GetRunID())
	cancel()
	require.NoError(t, err)

	// Wait for workflow to complete and verify it's canceled
	resultCtx, resultCancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer resultCancel()

	var result any
	err = w.Get(resultCtx, &result)

	// Should fail with a canceled error
	assert.Error(t, err)

	// Verify the workflow was actually canceled
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	we, err := s.Client.DescribeWorkflowExecution(ctx, w.GetID(), w.GetRunID())
	require.NoError(t, err)
	assert.Equal(t, enums.WORKFLOW_EXECUTION_STATUS_CANCELED, we.WorkflowExecutionInfo.Status)
	cancel()

	stopCh <- struct{}{}
	wg.Wait()
}
