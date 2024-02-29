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

const (
	addNameM             = "addName"
	addNameWOValidationM = "addNameWithoutValidation"
	throwExcM            = "throwException"
	randomizeNameM       = "randomizeName"
	addNameViaActivityM  = "addNameViaActivity"
	// signal
	exitSig = "exit"
	// WF names
	updateGreetWF = "Update.greet"
)

func Test_UpdatesInit(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		updateGreetWF)

	require.NoError(t, err)
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	handle, err := s.Client.UpdateWorkflow(ctx, w.GetID(), w.GetRunID(), addNameM, "John Doe")
	require.NoError(t, err)

	var result any

	err = handle.Get(context.Background(), &result)
	require.NoError(t, err)
	require.Equal(t, "Hello, John Doe!", result.(string))

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), exitSig, nil)
	require.NoError(t, err)

	time.Sleep(time.Second)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED
	})

	stopCh <- struct{}{}
	wg.Wait()
}

func Test_Updates_2(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		updateGreetWF)

	require.NoError(t, err)
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	handle, err := s.Client.UpdateWorkflow(ctx, w.GetID(), w.GetRunID(), addNameWOValidationM, "John Doe 42")
	require.NoError(t, err)

	var result any

	err = handle.Get(context.Background(), &result)
	require.NoError(t, err)
	require.Equal(t, "Hello, John Doe 42!", result.(string))

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), exitSig, nil)
	require.NoError(t, err)

	time.Sleep(time.Second)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED
	})

	stopCh <- struct{}{}
	wg.Wait()
}

func Test_Updates_4(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		updateGreetWF)

	require.NoError(t, err)
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	handle, err := s.Client.UpdateWorkflow(ctx, w.GetID(), w.GetRunID(), addNameM, "42")
	require.NoError(t, err)

	var result any

	err = handle.Get(context.Background(), &result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Name must not contain digits")

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), exitSig, nil)
	require.NoError(t, err)

	time.Sleep(time.Second)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED
	})

	stopCh <- struct{}{}
	wg.Wait()
}

func Test_Updates_5(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		updateGreetWF)

	require.NoError(t, err)
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	handle, err := s.Client.UpdateWorkflow(ctx, w.GetID(), w.GetRunID(), throwExcM, "John Doe")
	require.NoError(t, err)

	var result any

	err = handle.Get(context.Background(), &result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Test exception with John Doe")

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), exitSig, nil)
	require.NoError(t, err)

	time.Sleep(time.Second)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED
	})

	stopCh <- struct{}{}
	wg.Wait()
}

func Test_Updates_6(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		updateGreetWF)

	require.NoError(t, err)
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	handle, err := s.Client.UpdateWorkflow(ctx, w.GetID(), w.GetRunID(), randomizeNameM, 1)
	require.NoError(t, err)

	var result []any

	err = handle.Get(context.Background(), &result)
	require.NoError(t, err)
	require.Len(t, result, 1)

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), exitSig, nil)
	require.NoError(t, err)

	time.Sleep(time.Second)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED
	})

	stopCh <- struct{}{}
	wg.Wait()
}

func Test_Updates_7(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		updateGreetWF)

	require.NoError(t, err)
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	handle, err := s.Client.UpdateWorkflow(ctx, w.GetID(), w.GetRunID(), randomizeNameM, 3)
	require.NoError(t, err)

	var result []any

	err = handle.Get(context.Background(), &result)
	require.NoError(t, err)
	require.Len(t, result, 3)

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), exitSig, nil)
	require.NoError(t, err)

	time.Sleep(time.Second)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED
	})

	stopCh <- struct{}{}
	wg.Wait()
}

func Test_Updates_8(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		updateGreetWF)

	require.NoError(t, err)
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	handle, err := s.Client.UpdateWorkflow(ctx, w.GetID(), w.GetRunID(), addNameViaActivityM, "John Doe")
	require.NoError(t, err)

	var result any

	err = handle.Get(context.Background(), &result)
	require.NoError(t, err)
	require.Equal(t, "Hello, john doe!", result.(string))

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), exitSig, nil)
	require.NoError(t, err)

	time.Sleep(time.Second)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED
	})

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_ACTIVITY_TASK_COMPLETED
	})

	stopCh <- struct{}{}
	wg.Wait()
}
