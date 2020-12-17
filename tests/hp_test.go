package tests

import (
	"context"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"testing"
	"time"
)

func Test_VerifyRegistration(t *testing.T) {
	s := NewTestServer()
	defer s.MustClose()

	assert.Contains(t, s.workflows.WorkflowNames(), "SimpleWorkflow")
	assert.Contains(t, s.workflows.WorkflowNames(), "SimpleSignalledWorkflow")
	assert.Contains(t, s.workflows.WorkflowNames(), "ParallelScopesWorkflow")
	assert.Contains(t, s.workflows.WorkflowNames(), "TimerWorkflow")
	assert.Contains(t, s.workflows.WorkflowNames(), "SideEffectWorkflow")

	assert.Contains(t, s.activities.ActivityNames(), "SimpleActivity.echo")

	// todo: fix bug
	//assert.Contains(t, s.activities.ActivityNames(), "SimpleActivity.lower")
}

func Test_ExecuteSimpleWorkflow(t *testing.T) {
	s := NewTestServer()
	defer s.MustClose()

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "HELLO WORLD", result)
}

func Test_MultipleWorkflowsInSingleWorker(t *testing.T) {
	s := NewTestServer()
	defer s.MustClose()

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	w2, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "HELLO WORLD", result)

	assert.NoError(t, w2.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)
}

func Test_ExecuteWorkflowWithParallelScopes(t *testing.T) {
	s := NewTestServer()
	defer s.MustClose()

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"ParallelScopesWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "HELLO WORLD|Hello World|hello world", result)
}

func Test_Timer(t *testing.T) {
	s := NewTestServer()
	defer s.MustClose()

	start := time.Now()
	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)
	assert.True(t, time.Since(start).Seconds() > 1)

	s.AssertContainsEvent(t, w, func(event *history.HistoryEvent) bool {
		if event.EventType == enums.EVENT_TYPE_TIMER_STARTED {
			return true
		}

		return false
	})
}

func Test_SideEffect(t *testing.T) {
	s := NewTestServer()
	defer s.MustClose()

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SideEffectWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Contains(t, result, "hello world-")

	s.AssertContainsEvent(t, w, func(event *history.HistoryEvent) bool {
		if event.EventType == enums.EVENT_TYPE_MARKER_RECORDED {
			return true
		}

		return false
	})
}
