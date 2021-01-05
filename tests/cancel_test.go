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

func Test_CancellableWorkflowScope(t *testing.T) {
	s := NewTestServer()
	defer s.MustClose()

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"CancelledScopeWorkflow",
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
}

func Test_CancelledWorkflow(t *testing.T) {
	s := NewTestServer()
	defer s.MustClose()

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"CancelledWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	time.Sleep(time.Second)
	err = s.Client().CancelWorkflow(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	var result interface{}
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "OK", result)

	time.Sleep(5 * time.Second)

	e, err := s.Client().QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "getStatus")
	assert.NoError(t, err)

	trace := make([]string, 0)
	assert.NoError(t, e.Get(&trace))
	assert.Equal(
		t,
		[]string{
			"yield",
			"rollback",
			"START rollback",
			"WAIT ROLLBACK",
			"RESULT (ROLLBACK)", "DONE rollback",
			"COMPLETE rollback",
			"result: OK"},
		trace,
	)
}
