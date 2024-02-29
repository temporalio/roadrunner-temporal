package updates

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"tests/helpers"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	updatepb "go.temporal.io/api/update/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

const (
	awaitWithTimeoutM = "awaitWithTimeout"
	awaitM            = "await"
	resolveValueM     = "resolveValue"
	// signal
	exitSignal = "exit"
	// queryResult
	getValueQuery = "getValue"
	// WF names
	awaitsUpdateGreetWF = "AwaitsUpdate.greet"
)

func Test_Updates_9(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		awaitsUpdateGreetWF)

	require.NoError(t, err)
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	handle, err := s.Client.UpdateWorkflowWithOptions(ctx, &client.UpdateWorkflowWithOptionsRequest{
		RunID:      w.GetRunID(),
		WorkflowID: w.GetID(),
		UpdateName: awaitWithTimeoutM,
		Args:       []any{"key", 1, "fallback"},
		WaitPolicy: &updatepb.WaitPolicy{
			LifecycleStage: enums.UPDATE_WORKFLOW_EXECUTION_LIFECYCLE_STAGE_ACCEPTED,
		},
	})
	require.NoError(t, err)

	var result any

	err = handle.Get(context.Background(), &result)
	require.NoError(t, err)
	require.Equal(t, "fallback", result.(string))

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), exitSignal, nil)
	require.NoError(t, err)
	time.Sleep(time.Second)

	err = s.Client.GetWorkflow(ctx, w.GetID(), w.GetRunID()).Get(context.Background(), &result)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"key": "fallback"}, result.(map[string]any))

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED
	})

	stopCh <- struct{}{}
	wg.Wait()

	t.Cleanup(func() {
		_, errd := s.Client.WorkflowService().DeleteWorkflowExecution(context.Background(), &workflowservice.DeleteWorkflowExecutionRequest{
			Namespace: "default",
			WorkflowExecution: &common.WorkflowExecution{
				WorkflowId: w.GetID(),
				RunId:      w.GetRunID(),
			},
		})
		require.NoError(t, errd)
		s.Client.Close()
	})
}

func Test_Updates_10(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(2)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		awaitsUpdateGreetWF)

	require.NoError(t, err)
	time.Sleep(time.Second)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		handle, err2 := s.Client.UpdateWorkflowWithOptions(ctx, &client.UpdateWorkflowWithOptionsRequest{
			RunID:      w.GetRunID(),
			WorkflowID: w.GetID(),
			UpdateName: awaitM,
			Args:       []any{"key"},
			WaitPolicy: &updatepb.WaitPolicy{
				LifecycleStage: enums.UPDATE_WORKFLOW_EXECUTION_LIFECYCLE_STAGE_ACCEPTED,
			},
		})
		require.NoError(t, err2)

		var result any
		err2 = handle.Get(context.Background(), &result)
		require.NoError(t, err2)
		require.Equal(t, "resolved", result.(string))
		wg.Done()
	}()

	time.Sleep(time.Second * 3)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	queryResult, err := s.Client.QueryWorkflow(ctx, w.GetID(), w.GetRunID(), getValueQuery, "key")
	require.NoError(t, err)

	var queryRes string
	err = queryResult.Get(&queryRes)
	require.NoError(t, err)
	require.Equal(t, "", queryRes)

	ctx3, cancel3 := context.WithTimeout(context.Background(), time.Minute)
	defer cancel3()

	handle, err := s.Client.UpdateWorkflowWithOptions(ctx3, &client.UpdateWorkflowWithOptionsRequest{
		RunID:      w.GetRunID(),
		WorkflowID: w.GetID(),
		UpdateName: resolveValueM,
		Args:       []any{"key", "resolved"},
		WaitPolicy: &updatepb.WaitPolicy{
			LifecycleStage: enums.UPDATE_WORKFLOW_EXECUTION_LIFECYCLE_STAGE_ACCEPTED,
		},
	})
	require.NoError(t, err)

	var resultaw string
	err = handle.Get(context.Background(), &resultaw)
	require.NoError(t, err)
	require.Equal(t, "resolved", resultaw)

	queryResult2, err := s.Client.QueryWorkflow(ctx, w.GetID(), w.GetRunID(), getValueQuery, "key")
	require.NoError(t, err)

	var queryRes2 string
	err = queryResult2.Get(&queryRes2)
	require.NoError(t, err)
	require.Equal(t, "resolved", queryRes2)

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), exitSignal, nil)
	require.NoError(t, err)
	time.Sleep(time.Second)

	var wfresult any
	err = s.Client.GetWorkflow(ctx, w.GetID(), w.GetRunID()).Get(context.Background(), &wfresult)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"key": "resolved"}, wfresult.(map[string]any))

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED
	})

	stopCh <- struct{}{}
	wg.Wait()

	t.Cleanup(func() {
		_, errd := s.Client.WorkflowService().DeleteWorkflowExecution(context.Background(), &workflowservice.DeleteWorkflowExecutionRequest{
			Namespace: "default",
			WorkflowExecution: &common.WorkflowExecution{
				WorkflowId: w.GetID(),
				RunId:      w.GetRunID(),
			},
		})
		require.NoError(t, errd)
		s.Client.Close()
	})
}

func Test_Updates_11(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(11)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		awaitsUpdateGreetWF)

	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		go func(i int) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			handle, err2 := s.Client.UpdateWorkflowWithOptions(ctx, &client.UpdateWorkflowWithOptionsRequest{
				RunID:      w.GetRunID(),
				WorkflowID: w.GetID(),
				UpdateName: awaitM,
				Args:       []any{fmt.Sprintf("key-%d", i)},
				WaitPolicy: &updatepb.WaitPolicy{
					LifecycleStage: enums.UPDATE_WORKFLOW_EXECUTION_LIFECYCLE_STAGE_ACCEPTED,
				},
			})
			require.NoError(t, err2)

			var result any
			err2 = handle.Get(context.Background(), &result)
			require.NoError(t, err2)
			require.Equal(t, "resolved", result.(string))
			wg.Done()
		}(i)
	}

	time.Sleep(time.Second * 5)

	for i := 0; i < 5; i++ {
		go func(i int) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			handle, err2 := s.Client.UpdateWorkflowWithOptions(ctx, &client.UpdateWorkflowWithOptionsRequest{
				RunID:      w.GetRunID(),
				WorkflowID: w.GetID(),
				UpdateName: resolveValueM,
				Args:       []any{fmt.Sprintf("key-%d", i), "resolved"},
				WaitPolicy: &updatepb.WaitPolicy{
					LifecycleStage: enums.UPDATE_WORKFLOW_EXECUTION_LIFECYCLE_STAGE_ACCEPTED,
				},
			})
			require.NoError(t, err2)

			var result any
			err2 = handle.Get(context.Background(), &result)
			require.NoError(t, err2)
			require.Equal(t, "resolved", result.(string))
			wg.Done()
		}(i)
	}

	time.Sleep(time.Second * 3)

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), exitSignal, nil)
	require.NoError(t, err)

	time.Sleep(time.Second)

	var wfresult any
	err = s.Client.GetWorkflow(context.Background(), w.GetID(), w.GetRunID()).Get(context.Background(), &wfresult)
	require.NoError(t, err)
	res := map[string]any{"key-0": "resolved", "key-1": "resolved", "key-2": "resolved", "key-3": "resolved", "key-4": "resolved"}
	require.Equal(t, res, wfresult.(map[string]any))

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED
	})

	stopCh <- struct{}{}
	wg.Wait()

	t.Cleanup(func() {
		ctxCl, cancelCl := context.WithTimeout(context.Background(), time.Second*10)
		defer cancelCl()
		_, errd := s.Client.WorkflowService().DeleteWorkflowExecution(ctxCl, &workflowservice.DeleteWorkflowExecutionRequest{
			Namespace: "default",
			WorkflowExecution: &common.WorkflowExecution{
				WorkflowId: w.GetID(),
				RunId:      w.GetRunID(),
			},
		})
		require.NoError(t, errd)
		s.Client.Close()
	})
}

func Test_Updates_12(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(11)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		awaitsUpdateGreetWF)

	require.NoError(t, err)
	time.Sleep(time.Second)

	for i := 0; i < 5; i++ {
		go func(i int) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			handle, err2 := s.Client.UpdateWorkflowWithOptions(ctx, &client.UpdateWorkflowWithOptionsRequest{
				RunID:      w.GetRunID(),
				WorkflowID: w.GetID(),
				UpdateName: awaitM,
				Args:       []any{fmt.Sprintf("key-%d", i)},
				WaitPolicy: &updatepb.WaitPolicy{
					LifecycleStage: enums.UPDATE_WORKFLOW_EXECUTION_LIFECYCLE_STAGE_ACCEPTED,
				},
			})
			require.NoError(t, err2)

			var result any
			err2 = handle.Get(context.Background(), &result)
			require.NoError(t, err2)
			require.Equal(t, fmt.Sprintf("resolved-%d", i), result.(string))
			wg.Done()
		}(i)
	}

	time.Sleep(time.Second * 5)

	for i := 0; i < 5; i++ {
		go func(i int) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			handle, err2 := s.Client.UpdateWorkflowWithOptions(ctx, &client.UpdateWorkflowWithOptionsRequest{
				RunID:      w.GetRunID(),
				WorkflowID: w.GetID(),
				UpdateName: resolveValueM,
				Args:       []any{fmt.Sprintf("key-%d", i), fmt.Sprintf("resolved-%d", i)},
				WaitPolicy: &updatepb.WaitPolicy{
					LifecycleStage: enums.UPDATE_WORKFLOW_EXECUTION_LIFECYCLE_STAGE_ACCEPTED,
				},
			})
			require.NoError(t, err2)

			var result any
			err2 = handle.Get(context.Background(), &result)
			require.NoError(t, err2)
			require.Equal(t, fmt.Sprintf("resolved-%d", i), result.(string))
			wg.Done()
		}(i)
	}

	time.Sleep(time.Second * 3)

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), exitSignal, nil)
	require.NoError(t, err)
	time.Sleep(time.Second * 5)

	var wfresult any
	err = s.Client.GetWorkflow(context.Background(), w.GetID(), w.GetRunID()).Get(context.Background(), &wfresult)
	require.NoError(t, err)
	res := map[string]any{"key-0": "resolved-0", "key-1": "resolved-1", "key-2": "resolved-2", "key-3": "resolved-3", "key-4": "resolved-4"}
	require.Equal(t, res, wfresult.(map[string]any))

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED
	})

	stopCh <- struct{}{}

	wg.Wait()

	t.Cleanup(func() {
		_, errd := s.Client.WorkflowService().DeleteWorkflowExecution(context.Background(), &workflowservice.DeleteWorkflowExecutionRequest{
			Namespace: "default",
			WorkflowExecution: &common.WorkflowExecution{
				WorkflowId: w.GetID(),
				RunId:      w.GetRunID(),
			},
		})
		require.NoError(t, errd)
		s.Client.Close()
	})
}
