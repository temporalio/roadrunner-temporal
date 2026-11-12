package tests

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"tests/helpers"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	nexuspb "go.temporal.io/api/nexus/v1"
	"go.temporal.io/api/operatorservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

func Test_NexusOperationFromWorkflow(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-nexus.yaml")
	endpoint := createNexusEndpoint(t, s)

	startCaller := func(t *testing.T, operation string) client.WorkflowRun {
		w, err := s.Client.ExecuteWorkflow(
			t.Context(),
			client.StartWorkflowOptions{TaskQueue: "nexus"},
			"NexusCallerWorkflow",
			endpoint,
			operation,
			"world",
		)
		require.NoError(t, err)
		return w
	}

	isOperationStarted := func(e *history.HistoryEvent) bool {
		return e.GetEventType() == enums.EVENT_TYPE_NEXUS_OPERATION_STARTED
	}

	t.Run("sync", func(t *testing.T) {
		w := startCaller(t, "greet")

		var result string
		require.NoError(t, w.Get(t.Context(), &result))
		assert.Equal(t, "Hello, world!", result)
		s.AssertNotContainsEvent(s.Client, t, w, isOperationStarted)
	})

	t.Run("async", func(t *testing.T) {
		w := startCaller(t, "greetAsync")

		var result string
		require.NoError(t, w.Get(t.Context(), &result))
		assert.Equal(t, "Hello async, world!", result)
		s.AssertContainsEvent(s.Client, t, w, isOperationStarted)
	})

	t.Run("failure", func(t *testing.T) {
		w := startCaller(t, "fail")

		err := w.Get(t.Context(), nil)
		var nexusErr *temporal.NexusOperationError
		require.ErrorAs(t, err, &nexusErr)
		assert.Equal(t, "GreetingService", nexusErr.Service)
		assert.Equal(t, "fail", nexusErr.Operation)
		var appErr *temporal.ApplicationError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, "Cannot greet world", appErr.Message())
	})

	t.Run("cancel", func(t *testing.T) {
		w := startCaller(t, "waitForCancel")

		var requestID string
		require.Eventually(t, func() bool {
			requestID = ""
			started := false
			events := s.Client.GetWorkflowHistory(t.Context(), w.GetID(), w.GetRunID(), false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
			for events.HasNext() {
				e, err := events.Next()
				require.NoError(t, err)
				if attrs := e.GetNexusOperationScheduledEventAttributes(); attrs != nil {
					requestID = attrs.GetRequestId()
				}
				started = started || isOperationStarted(e)
			}
			return started
		}, 10*time.Second, 100*time.Millisecond)

		require.NoError(t, s.Client.CancelWorkflow(t.Context(), w.GetID(), w.GetRunID()))

		var canceledErr *temporal.CanceledError
		require.ErrorAs(t, w.Get(t.Context(), nil), &canceledErr)

		handler, err := s.Client.DescribeWorkflowExecution(t.Context(), requestID, "")
		require.NoError(t, err)
		assert.Equal(t, enums.WORKFLOW_EXECUTION_STATUS_CANCELED, handler.GetWorkflowExecutionInfo().GetStatus())
	})

	stopCh <- struct{}{}
	wg.Wait()
}

func createNexusEndpoint(t *testing.T, s *helpers.TestServer) string {
	name := "rr-nexus-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	created, err := s.Client.OperatorService().CreateNexusEndpoint(t.Context(), &operatorservice.CreateNexusEndpointRequest{
		Spec: &nexuspb.EndpointSpec{
			Name: name,
			Target: &nexuspb.EndpointTarget{
				Variant: &nexuspb.EndpointTarget_Worker_{
					Worker: &nexuspb.EndpointTarget_Worker{Namespace: "default", TaskQueue: "nexus"},
				},
			},
		},
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, err := s.Client.OperatorService().DeleteNexusEndpoint(context.Background(), &operatorservice.DeleteNexusEndpointRequest{
			Id:      created.GetEndpoint().GetId(),
			Version: created.GetEndpoint().GetVersion(),
		})
		assert.NoError(t, err)
	})

	return name
}
