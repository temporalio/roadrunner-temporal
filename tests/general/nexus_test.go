package tests

import (
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
)

func Test_NexusOperationFromWorkflow(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-nexus.yaml")

	endpoint := "rr-nexus-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	created, err := s.Client.OperatorService().CreateNexusEndpoint(t.Context(), &operatorservice.CreateNexusEndpointRequest{
		Spec: &nexuspb.EndpointSpec{
			Name: endpoint,
			Target: &nexuspb.EndpointTarget{
				Variant: &nexuspb.EndpointTarget_Worker_{
					Worker: &nexuspb.EndpointTarget_Worker{Namespace: "default", TaskQueue: "nexus"},
				},
			},
		},
	})
	require.NoError(t, err)

	isOperationStarted := func(e *history.HistoryEvent) bool {
		return e.GetEventType() == enums.EVENT_TYPE_NEXUS_OPERATION_STARTED
	}

	cases := map[string]struct {
		async bool
		want  string
	}{
		"sync":  {async: false, want: "Hello, world!"},
		"async": {async: true, want: "Hello async, world!"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w, err := s.Client.ExecuteWorkflow(
				t.Context(),
				client.StartWorkflowOptions{TaskQueue: "nexus"},
				"NexusCallerWorkflow",
				endpoint,
				"world",
				tc.async,
			)
			require.NoError(t, err)

			var result string
			require.NoError(t, w.Get(t.Context(), &result))
			assert.Equal(t, tc.want, result)

			if tc.async {
				s.AssertContainsEvent(s.Client, t, w, isOperationStarted)
			} else {
				s.AssertNotContainsEvent(s.Client, t, w, isOperationStarted)
			}
		})
	}

	_, err = s.Client.OperatorService().DeleteNexusEndpoint(t.Context(), &operatorservice.DeleteNexusEndpointRequest{
		Id:      created.GetEndpoint().GetId(),
		Version: created.GetEndpoint().GetVersion(),
	})
	assert.NoError(t, err)

	stopCh <- struct{}{}
	wg.Wait()
}
