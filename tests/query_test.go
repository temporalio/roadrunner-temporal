package tests

import (
	"context"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/client"
	"testing"
	"time"
)

func Test_ListQueries(t *testing.T) {
	s := NewTestServer()
	defer s.MustClose()

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"QueryWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	time.Sleep(time.Millisecond * 500)

	v, err := s.Client().QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "error", -1)
	assert.Nil(t, v)
	assert.Error(t, err)

	assert.Contains(t, err.Error(), "KnownQueryTypes=[get]")
}

func Test_GetQuery(t *testing.T) {
	s := NewTestServer()
	defer s.MustClose()

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"QueryWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	err = s.Client().SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), "add", 88)
	assert.NoError(t, err)
	time.Sleep(time.Millisecond * 500)

	v, err := s.Client().QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "get", nil)
	assert.NoError(t, err)

	var r int
	assert.NoError(t, v.Get(&r))
	assert.Equal(t, 88, r)
}

//
//func Test_SendSignalDuringTimer(t *testing.T) {
//	s := NewTestServer()
//	defer s.MustClose()
//
//	w, err := s.Client().SignalWithStartWorkflow(
//		context.Background(),
//		"signalled-"+uuid.New(),
//		"add",
//		10,
//		client.StartWorkflowOptions{
//			TaskQueue: "default",
//		},
//		"SimpleSignalledWorkflow",
//	)
//	assert.NoError(t, err)
//
//	err = s.Client().SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), "add", -1)
//	assert.NoError(t, err)
//
//	var result int
//	assert.NoError(t, w.Get(context.Background(), &result))
//	assert.Equal(t, 9, result)
//
//	s.AssertContainsEvent(t, w, func(event *history.HistoryEvent) bool {
//		if event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED {
//			attr := event.Attributes.(*history.HistoryEvent_WorkflowExecutionSignaledEventAttributes)
//			return attr.WorkflowExecutionSignaledEventAttributes.SignalName == "add"
//		}
//
//		return false
//	})
//}
//
//func Test_SendSignalBeforeCompletingWorkflow(t *testing.T) {
//	s := NewTestServer()
//	defer s.MustClose()
//
//	w, err := s.Client().ExecuteWorkflow(
//		context.Background(),
//		client.StartWorkflowOptions{
//			TaskQueue: "default",
//		},
//		"SimpleSignalledWorkflowWithSleep",
//	)
//	assert.NoError(t, err)
//
//	// should be around sleep(1) call
//	time.Sleep(time.Second + time.Millisecond*500)
//
//	err = s.Client().SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), "add", -1)
//	assert.NoError(t, err)
//
//	var result int
//	assert.NoError(t, w.Get(context.Background(), &result))
//	assert.Equal(t, -1, result)
//
//	s.AssertContainsEvent(t, w, func(event *history.HistoryEvent) bool {
//		if event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED {
//			attr := event.Attributes.(*history.HistoryEvent_WorkflowExecutionSignaledEventAttributes)
//			return attr.WorkflowExecutionSignaledEventAttributes.SignalName == "add"
//		}
//
//		return false
//	})
//}
