package updates

import (
	"context"
	"sync"
	"testing"
	"tests/helpers"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
)

func Test_UpdatesInit(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"Update.greet")

	require.NoError(t, err)
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	handle, err := s.Client.UpdateWorkflow(ctx, w.GetID(), w.GetRunID(), "addName", "John Doe")
	require.NoError(t, err)

	var result any

	err = handle.Get(context.Background(), &result)
	require.NoError(t, err)
	require.Equal(t, "Hello, John Doe!", result.(string))

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), "exit", nil)
	require.NoError(t, err)

	time.Sleep(time.Second)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED
	})

	stopCh <- struct{}{}
	wg.Wait()
}

func Test_UpdatesSideEffect(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"Update.greet")

	require.NoError(t, err)
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	handle, err := s.Client.UpdateWorkflow(ctx, w.GetID(), w.GetRunID(), "randomizeName", 3)
	require.NoError(t, err)

	var result []any

	err = handle.Get(context.Background(), &result)
	require.NoError(t, err)
	require.Len(t, result, 3)

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), "exit", nil)
	require.NoError(t, err)

	time.Sleep(time.Second)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED
	})

	stopCh <- struct{}{}
	wg.Wait()
}
