package tests

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	configImpl "github.com/roadrunner-server/config/v4"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/client"
)

func Test_SimpleWorkflowMetrics(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cfg := &configImpl.Plugin{
		Timeout: time.Second * 30,
	}
	cfg.Path = "configs/.rr-metrics.yaml"
	cfg.Prefix = "rr"
	cfg.Version = "2.9.0"

	s := NewTestServerWithMetrics(t, stopCh, cfg, wg)

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"WithChildStubWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "Child: CHILD HELLO WORLD", result)

	we, err := s.Client.DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
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
	assert.Contains(t, metrics, "samples_rr_activities_pool_queue_size")
	assert.Contains(t, metrics, "samples_rr_workflows_pool_queue_size")

	assert.Contains(t, metrics, "workflow_task_queue_poll_succeed")
	assert.Contains(t, metrics, "workflow_task_replay_latency")
	assert.Contains(t, metrics, "workflow_task_schedule_to_start_latency")

	assert.Equal(t, "Completed", we.WorkflowExecutionInfo.Status.String())
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_SimpleWorkflowMetricsPrometheusNewDriver(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cfg := &configImpl.Plugin{
		Timeout: time.Second * 30,
	}
	cfg.Path = "configs/.rr-metrics-prom-new.yaml"
	cfg.Prefix = "rr"
	cfg.Version = "2.11.2"

	s := NewTestServerWithMetrics(t, stopCh, cfg, wg)

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"WithChildStubWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "Child: CHILD HELLO WORLD", result)

	we, err := s.Client.DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
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
	assert.Contains(t, metrics, "samples_rr_activities_pool_queue_size")
	assert.Contains(t, metrics, "samples_rr_workflows_pool_queue_size")

	assert.Contains(t, metrics, "workflow_task_queue_poll_succeed")
	assert.Contains(t, metrics, "workflow_task_replay_latency")
	assert.Contains(t, metrics, "workflow_task_schedule_to_start_latency")

	assert.Equal(t, "Completed", we.WorkflowExecutionInfo.Status.String())
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_SimpleWorkflowMetricsStatsdNewDriver(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cfg := &configImpl.Plugin{
		Timeout: time.Second * 30,
	}
	cfg.Path = "configs/.rr-metrics-statsd.yaml"
	cfg.Prefix = "rr"
	cfg.Version = "2.11.2"

	s := NewTestServerWithMetrics(t, stopCh, cfg, wg)

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"WithChildStubWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "Child: CHILD HELLO WORLD", result)

	we, err := s.Client.DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	time.Sleep(time.Second * 2)
	metrics, err := getStatsd()
	assert.NoError(t, err)

	assert.Contains(t, metrics, "request_attempt")
	assert.Contains(t, metrics, "long_request_attempt")
	assert.Contains(t, metrics, "poller_start")
	assert.Contains(t, metrics, "request_attempt")
	assert.Contains(t, metrics, "request")
	assert.Contains(t, metrics, "worker_start")
	assert.Contains(t, metrics, "workflow_task_queue_poll_succeed")

	assert.Equal(t, "Completed", we.WorkflowExecutionInfo.Status.String())
	stopCh <- struct{}{}
	wg.Wait()
}

// get request and return body
func get() (string, error) {
	r, err := http.Get("http://127.0.0.1:9095/metrics")
	if err != nil {
		return "", err
	}

	b, err := io.ReadAll(r.Body)
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

// get request and return body
func getStatsd() (string, error) {
	conn, err := net.Dial("tcp4", "127.0.0.1:8126")
	if err != nil {
		panic(err)
	}

	_, err = conn.Write([]byte("counters"))
	if err != nil {
		panic(err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(time.Second * 2))
	d, _ := io.ReadAll(conn)
	return string(d), nil
}
