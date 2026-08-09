package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	"tests/helpers"

	"github.com/stretchr/testify/require"
	"go.temporal.io/api/workflowservice/v1"
)

// End-to-end: a worker booted with worker_heartbeat_interval actually sends
// heartbeats, so the server reports it via ListWorkers.
func Test_WorkerHeartbeat_ReportsRunningWorker(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-worker-heartbeat.yaml")

	// Interval is 1s; poll ListWorkers until our task queue's worker shows up.
	require.Eventually(t, func() bool {
		resp, err := s.Client.WorkflowService().ListWorkers(context.Background(), &workflowservice.ListWorkersRequest{
			Namespace: "default",
			PageSize:  100,
		})
		if err != nil {
			return false
		}
		for _, w := range resp.GetWorkersInfo() {
			if hb := w.GetWorkerHeartbeat(); hb != nil && hb.GetTaskQueue() == "default" {
				return true
			}
		}
		return false
	}, 15*time.Second, time.Second, "server should report a heartbeating worker on the default task queue")

	stopCh <- struct{}{}
	wg.Wait()
}
