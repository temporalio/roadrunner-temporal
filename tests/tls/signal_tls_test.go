package tls

import (
	"context"
	"sync"
	"testing"
	"time"

	"tests/helpers"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
)

const (
	signalStr = "signaled-"
	addStr    = "add"
)

func Test_SignalsWithoutSignalsProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServerTLS(t, stopCh, wg, ".rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleSignaledWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result int
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, 0, result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_SendSignalDuringTimerProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServerTLS(t, stopCh, wg, ".rr-proto.yaml")

	w, err := s.Client.SignalWithStartWorkflow(
		context.Background(),
		signalStr+uuid.New(),
		addStr,
		10,
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleSignaledWorkflow",
	)
	assert.NoError(t, err)

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), addStr, -1)
	assert.NoError(t, err)

	var result int
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, 9, result)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		if event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED {
			attr := event.Attributes.(*history.HistoryEvent_WorkflowExecutionSignaledEventAttributes)
			return attr.WorkflowExecutionSignaledEventAttributes.SignalName == addStr
		}

		return false
	})
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_SendSignalBeforeCompletingWorkflowProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServerTLS(t, stopCh, wg, ".rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleSignaledWorkflowWithSleep",
	)
	assert.NoError(t, err)

	// should be around sleep(5) call
	time.Sleep(time.Second)

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), addStr, -1)
	assert.NoError(t, err)

	var result int
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	assert.NoError(t, w.Get(ctx, &result))
	assert.Equal(t, -1, result)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		if event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED {
			attr := event.Attributes.(*history.HistoryEvent_WorkflowExecutionSignaledEventAttributes)
			return attr.WorkflowExecutionSignaledEventAttributes.SignalName == addStr
		}

		return false
	})
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_RuntimeSignalProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServerTLS(t, stopCh, wg, ".rr-proto.yaml")

	w, err := s.Client.SignalWithStartWorkflow(
		context.Background(),
		signalStr+uuid.New(),
		addStr,
		-1,
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"RuntimeSignalWorkflow",
	)
	assert.NoError(t, err)

	var result int
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, -1, result)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		if event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED {
			attr := event.Attributes.(*history.HistoryEvent_WorkflowExecutionSignaledEventAttributes)
			return attr.WorkflowExecutionSignaledEventAttributes.SignalName == addStr
		}

		return false
	})
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_SignalStepsProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServerTLS(t, stopCh, wg, ".rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"WorkflowWithSignaledSteps",
	)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	err = s.Client.SignalWorkflow(ctx, w.GetID(), w.GetRunID(), "begin", true)
	assert.NoError(t, err)

	err = s.Client.SignalWorkflow(ctx, w.GetID(), w.GetRunID(), "next1", true)
	assert.NoError(t, err)

	v, err := s.Client.QueryWorkflow(ctx, w.GetID(), w.GetRunID(), "value", nil)
	assert.NoError(t, err)

	var r int
	assert.NoError(t, v.Get(&r))
	assert.Equal(t, 2, r)

	err = s.Client.SignalWorkflow(ctx, w.GetID(), w.GetRunID(), "next2", true)
	assert.NoError(t, err)

	v, err = s.Client.QueryWorkflow(ctx, w.GetID(), w.GetRunID(), "value", nil)
	assert.NoError(t, err)

	assert.NoError(t, v.Get(&r))
	assert.Equal(t, 3, r)

	var result int
	assert.NoError(t, w.Get(ctx, &result))

	// 3 ticks
	assert.Equal(t, 3, result)
	stopCh <- struct{}{}
	wg.Wait()
}

// ----- LA

func Test_SignalsWithoutSignalsLAProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServerTLS(t, stopCh, wg, ".rr-proto-la.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleSignaledWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result int
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, 0, result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_SendSignalDuringTimerLAProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServerTLS(t, stopCh, wg, ".rr-proto-la.yaml")

	w, err := s.Client.SignalWithStartWorkflow(
		context.Background(),
		signalStr+uuid.New(),
		addStr,
		10,
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleSignaledWorkflow",
	)
	assert.NoError(t, err)

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), addStr, -1)
	assert.NoError(t, err)

	var result int
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, 9, result)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		if event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED {
			attr := event.Attributes.(*history.HistoryEvent_WorkflowExecutionSignaledEventAttributes)
			return attr.WorkflowExecutionSignaledEventAttributes.SignalName == addStr
		}

		return false
	})
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_SendSignalBeforeCompletingWorkflowLAProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServerTLS(t, stopCh, wg, ".rr-proto-la.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleSignaledWorkflowWithSleep",
	)
	assert.NoError(t, err)

	// should be around sleep(1) call
	time.Sleep(time.Second + time.Millisecond*200)

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), addStr, -1)
	assert.NoError(t, err)

	var result int
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, -1, result)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		if event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED {
			attr := event.Attributes.(*history.HistoryEvent_WorkflowExecutionSignaledEventAttributes)
			return attr.WorkflowExecutionSignaledEventAttributes.SignalName == addStr
		}

		return false
	})
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_RuntimeSignalLAProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServerTLS(t, stopCh, wg, ".rr-proto-la.yaml")

	w, err := s.Client.SignalWithStartWorkflow(
		context.Background(),
		signalStr+uuid.New(),
		addStr,
		-1,
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"RuntimeSignalWorkflow",
	)
	assert.NoError(t, err)

	var result int
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, -1, result)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		if event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED {
			attr := event.Attributes.(*history.HistoryEvent_WorkflowExecutionSignaledEventAttributes)
			return attr.WorkflowExecutionSignaledEventAttributes.SignalName == addStr
		}

		return false
	})
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_SignalStepsLAProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServerTLS(t, stopCh, wg, ".rr-proto-la.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"WorkflowWithSignaledSteps",
	)
	assert.NoError(t, err)

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), "begin", true)
	assert.NoError(t, err)

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), "next1", true)
	assert.NoError(t, err)

	v, err := s.Client.QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "value", nil)
	assert.NoError(t, err)

	var r int
	assert.NoError(t, v.Get(&r))
	assert.Equal(t, 2, r)

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), "next2", true)
	assert.NoError(t, err)

	v, err = s.Client.QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "value", nil)
	assert.NoError(t, err)

	assert.NoError(t, v.Get(&r))
	assert.Equal(t, 3, r)

	var result int
	assert.NoError(t, w.Get(context.Background(), &result))

	// 3 ticks
	assert.Equal(t, 3, result)
	stopCh <- struct{}{}
	wg.Wait()
}
