package tests

import (
	"context"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"tests/helpers"

	"github.com/stretchr/testify/require"
	workerpb "go.temporal.io/api/worker/v1"
	"go.temporal.io/api/workflowservice/v1"
	temporalClient "go.temporal.io/sdk/client"
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
		for _, w := range resp.GetWorkers() {
			if w.GetTaskQueue() == "default" {
				return true
			}
		}
		return false
	}, 15*time.Second, time.Second, "server should report a heartbeating worker on the default task queue")

	stopCh <- struct{}{}
	wg.Wait()
}

// defaultQueueHeartbeat returns the heartbeat this test process reported for the "default"
// task queue, or nil if the server has not seen one yet. ListWorkers returns only limited
// worker info, so the full heartbeat is fetched via DescribeWorker. RR runs in-process here,
// so the PID filter keeps stale heartbeats from earlier runs against a long-lived server out.
func defaultQueueHeartbeat(c temporalClient.Client) *workerpb.WorkerHeartbeat {
	resp, err := c.WorkflowService().ListWorkers(context.Background(), &workflowservice.ListWorkersRequest{
		Namespace: "default",
		PageSize:  100,
	})
	if err != nil {
		return nil
	}

	pid := strconv.Itoa(os.Getpid())
	for _, w := range resp.GetWorkers() {
		if w.GetTaskQueue() != "default" || w.GetProcessId() != pid {
			continue
		}

		desc, err := c.WorkflowService().DescribeWorker(context.Background(), &workflowservice.DescribeWorkerRequest{
			Namespace:         "default",
			WorkerInstanceKey: w.GetWorkerInstanceKey(),
		})
		if err != nil {
			return nil
		}

		return desc.GetWorkerInfo().GetWorkerHeartbeat()
	}

	return nil
}

// End-to-end: heartbeats carry host CPU/memory usage via WorkerOptions.SysInfoProvider.
// Without it the SDK reports zeros and the Temporal UI Workers panel shows "Missing Dependency".
func Test_WorkerHeartbeat_ReportsHostResourceUsage(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-worker-heartbeat.yaml")

	// Interval is 1s; poll until a heartbeat carrying host memory usage shows up.
	require.Eventually(t, func() bool {
		return defaultQueueHeartbeat(s.Client).GetHostInfo().GetCurrentHostMemUsage() > 0
	}, 15*time.Second, time.Second, "server should report host resource usage for the heartbeating worker")

	// Re-fetch outside the Eventually closure to stay race-free under -race.
	hi := defaultQueueHeartbeat(s.Client).GetHostInfo()
	require.NotNil(t, hi)

	// The provider reports fractions of total, not percentages.
	require.Greater(t, hi.GetCurrentHostMemUsage(), float32(0))
	require.LessOrEqual(t, hi.GetCurrentHostMemUsage(), float32(1))
	// CPU can legitimately sample as 0 on an idle host, so only bound it.
	require.GreaterOrEqual(t, hi.GetCurrentHostCpuUsage(), float32(0))
	require.LessOrEqual(t, hi.GetCurrentHostCpuUsage(), float32(1))

	stopCh <- struct{}{}
	wg.Wait()
}
