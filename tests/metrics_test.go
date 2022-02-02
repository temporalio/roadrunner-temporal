package tests

import (
	"context"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/client"
)

func Test_SimpleWorkflowCancelMetrics(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServerWithMetrics(t, stopCh, wg)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleSignaledWorkflow")
	assert.NoError(t, err)

	time.Sleep(time.Millisecond * 500)
	err = s.Client().CancelWorkflow(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	var result interface{}
	assert.Error(t, w.Get(context.Background(), &result))

	we, err := s.Client().DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	metrics, err := get()
	assert.NoError(t, err)

	assert.Contains(t, metrics, "request_attempt")
	assert.Contains(t, metrics, "schedule_to_start_latency")
	assert.Contains(t, metrics, "long_request_attempt")
	assert.Contains(t, metrics, "long_request_latency")
	assert.Contains(t, metrics, "long_request_latency_attempt")
	assert.Contains(t, metrics, "poller_start")
	assert.Contains(t, metrics, "request_attempt")
	assert.Contains(t, metrics, "request")
	assert.Contains(t, metrics, "request_latency_attempt")
	assert.Contains(t, metrics, "sticky_cache_size")
	assert.Contains(t, metrics, "worker_start")
	assert.Contains(t, metrics, "workflow_endtoend_latency")
	assert.Contains(t, metrics, "workflow_task_execution_latency")
	assert.Contains(t, metrics, "workflow_task_execution_latency_sum")

	assert.Contains(t, metrics, "workflow_task_queue_poll_succeed")
	assert.Contains(t, metrics, "workflow_task_replay_latency")
	assert.Contains(t, metrics, "workflow_task_schedule_to_start_latency")

	assert.Equal(t, "Canceled", we.WorkflowExecutionInfo.Status.String())
	stopCh <- struct{}{}
	wg.Wait()
}

// get request and return body
func get() (string, error) {
	r, err := http.Get("http://127.0.0.1:9095/metrics")
	if err != nil {
		return "", err
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	err = r.Body.Close()
	if err != nil {
		return "", err
	}
	// unsafe
	return string(b), err
}
