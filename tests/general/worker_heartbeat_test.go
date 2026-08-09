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

func defaultQueueHeartbeat(c temporalClient.Client) *workerpb.WorkerHeartbeat {
	pid := strconv.Itoa(os.Getpid())

	var newest *workerpb.WorkerListInfo
	var pageToken []byte
	for {
		resp, err := c.WorkflowService().ListWorkers(context.Background(), &workflowservice.ListWorkersRequest{
			Namespace:     "default",
			PageSize:      100,
			NextPageToken: pageToken,
		})
		if err != nil {
			return nil
		}

		for _, w := range resp.GetWorkers() {
			if w.GetTaskQueue() != "default" || w.GetProcessId() != pid {
				continue
			}
			if newest == nil || w.GetStartTime().AsTime().After(newest.GetStartTime().AsTime()) {
				newest = w
			}
		}

		pageToken = resp.GetNextPageToken()
		if len(pageToken) == 0 {
			break
		}
	}

	if newest == nil {
		return nil
	}

	desc, err := c.WorkflowService().DescribeWorker(context.Background(), &workflowservice.DescribeWorkerRequest{
		Namespace:         "default",
		WorkerInstanceKey: newest.GetWorkerInstanceKey(),
	})
	if err != nil {
		return nil
	}

	return desc.GetWorkerInfo().GetWorkerHeartbeat()
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
