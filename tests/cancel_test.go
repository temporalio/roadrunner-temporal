package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
)

func Test_SimpleWorkflowCancelProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleSignaledWorkflow")
	assert.NoError(t, err)

	time.Sleep(time.Millisecond * 500)
	err = s.Client().CancelWorkflow(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	var result interface{}
	assert.Error(t, w.Get(context.Background(), &result))

	we, err := s.Client().DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	assert.Equal(t, "Canceled", we.WorkflowExecutionInfo.Status.String())
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_CancellableWorkflowScopeProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"CanceledScopeWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "yes", result)

	s.AssertContainsEvent(t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_TIMER_CANCELED
	})

	s.AssertNotContainsEvent(t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED
	})
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_CanceledWorkflowProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"CanceledWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	time.Sleep(time.Second)
	err = s.Client().CancelWorkflow(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	var result interface{}
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "CANCELED", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_CanceledWithCompensationWorkflowProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"CanceledWithCompensationWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	err = s.Client().CancelWorkflow(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	var result interface{}
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "OK", result)

	e, err := s.Client().QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "getStatus")
	require.NoError(t, err)
	require.NotNil(t, e)

	trace := make([]string, 0)
	assert.NoError(t, e.Get(&trace))
	assert.Equal(
		t,
		[]string{
			"yield",
			"rollback",
			"captured retry",
			"captured promise on canceled",
			"START rollback",
			"WAIT ROLLBACK",
			"RESULT (ROLLBACK)", "DONE rollback",
			"COMPLETE rollback",
			"result: OK",
		},
		trace,
	)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_CanceledNestedWorkflowProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"CanceledNestedWorkflow",
	)
	assert.NoError(t, err)

	time.Sleep(time.Second)

	err = s.Client().CancelWorkflow(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	var result interface{}
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "CANCELED", result)

	e, err := s.Client().QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "getStatus")
	assert.NoError(t, err)

	trace := make([]string, 0)
	assert.NoError(t, e.Get(&trace))
	assert.Equal(
		t,
		[]string{
			"begin",
			"first scope",
			"second scope",
			"close second scope",
			"close first scope",
			"second scope canceled",
			"first scope canceled",
			"close process",
		},
		trace,
	)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_CanceledNSingleScopeWorkflowProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"CanceledSingleScopeWorkflow",
	)
	assert.NoError(t, err)

	time.Sleep(time.Second)

	err = s.Client().CancelWorkflow(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	var result interface{}
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "OK", result)

	e, err := s.Client().QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "getStatus")
	require.NoError(t, err)

	trace := make([]string, 0)
	assert.NoError(t, e.Get(&trace))
	assert.Equal(
		t,
		[]string{
			"start",
			"in scope",
			"on cancel",
			"captured in scope",
			"captured in process",
		},
		trace,
	)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_CanceledMidflightWorkflowProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"CanceledMidflightWorkflow",
	)
	assert.NoError(t, err)
	assert.NotNil(t, w)

	var result interface{}
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "OK", result)

	e, err := s.Client().QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "getStatus")
	assert.NoError(t, err)
	assert.NotNil(t, e)

	trace := make([]string, 0)

	assert.NoError(t, e.Get(&trace))
	assert.Equal(
		t,
		[]string{
			"start",
			"in scope",
			"on cancel",
			"done cancel",
		},
		trace,
	)

	s.AssertNotContainsEvent(t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED
	})
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_CancelSignaledChildWorkflowProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"CancelSignaledChildWorkflow",
	)
	assert.NoError(t, err)

	var result interface{}
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "canceled ok", result)

	e, err := s.Client().QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "getStatus")
	assert.NoError(t, err)

	trace := make([]string, 0)
	assert.NoError(t, e.Get(&trace))
	assert.Equal(
		t,
		[]string{
			"start",
			"child started",
			"child signaled",
			"scope canceled",
			"process done",
		},
		trace,
	)

	s.AssertContainsEvent(t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_REQUEST_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_INITIATED
	})
	stopCh <- struct{}{}
	wg.Wait()
}
