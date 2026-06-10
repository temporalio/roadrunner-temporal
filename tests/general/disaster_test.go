package tests

import (
	"context"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"tests/helpers"

	informerV1 "github.com/roadrunner-server/api-go/v6/informer/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
)

func Test_WorkerError_DisasterRecovery(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	workers := getWorkers(t)
	require.Len(t, workers, 5)

	p, err := os.FindProcess(int(workers[0].Pid))
	assert.NoError(t, err)

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	time.Sleep(time.Millisecond * 750)

	// must fully recover with new worker
	assert.NoError(t, p.Kill())

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ResetAll(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)
	time.Sleep(time.Millisecond * 750)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)

	reset(t)

	w, err = s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)
	time.Sleep(time.Millisecond * 750)

	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)

	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ResetWFWorker(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)
	time.Sleep(time.Millisecond * 750)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)

	wrks := getWorkers(t)

	for i := 0; i < len(wrks); i++ {
		_ = syscall.Kill(int(wrks[i].Pid), syscall.SIGKILL)
		time.Sleep(time.Second * 2)
	}

	time.Sleep(time.Second * 10)

	w, err = s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)
	time.Sleep(time.Millisecond * 750)

	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)

	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ActivityError_DisasterRecovery(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	defer func() {
		// always restore script
		_ = os.Rename("../php_test_files/worker.bak", "../php_test_files/worker.php")
	}()

	// Makes worker pool unable to recover for some time
	_ = os.Rename("../php_test_files/worker.php", "../php_test_files/worker.bak")

	// destroys all workers in activities

	workers := getWorkers(t)
	require.Len(t, workers, 5)

	for i := 1; i < len(workers); i++ {
		p, err := os.FindProcess(int(workers[i].Pid))
		require.NoError(t, err)
		require.NoError(t, p.Kill())
	}

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	// activity can't complete at this moment
	time.Sleep(time.Millisecond * 750)

	// restore the script and recover activity pool
	_ = os.Rename("../php_test_files/worker.bak", "../php_test_files/worker.php")

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "HELLO WORLD", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_WorkerError_DisasterRecoveryProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	workers := getWorkers(t)
	require.Len(t, workers, 5)

	p, err := os.FindProcess(int(workers[0].Pid))
	assert.NoError(t, err)

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	time.Sleep(time.Millisecond * 750)

	// must fully recover with new worker
	assert.NoError(t, p.Kill())

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_WorkerError_DisasterRecovery_Heavy(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	defer func() {
		// always restore script
		_ = os.Rename("../php_test_files/worker.bak", "../php_test_files/worker.php")
	}()

	// Makes worker pool unable to recover for some time
	require.NoError(t, os.Rename("../php_test_files/worker.php", "../php_test_files/worker.bak"))

	list, err := helpers.Workers(t.Context())
	require.NoError(t, err)

	p, err := os.FindProcess(int(list[0].GetPid()))
	assert.NoError(t, err)

	// must fully recover with new worker
	assert.NoError(t, p.Kill())

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	time.Sleep(time.Second * 5)

	// restore the script and recover activity pool
	_ = os.Rename("../php_test_files/worker.bak", "../php_test_files/worker.php")

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ActivityError_DisasterRecoveryProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	defer func() {
		// always restore script
		_ = os.Rename("../php_test_files/worker.bak", "../php_test_files/worker.php")
	}()

	// Makes worker pool unable to recover for some time
	_ = os.Rename("../php_test_files/worker.php", "../php_test_files/worker.bak")

	// destroys all workers in activities
	workers := getWorkers(t)
	require.Len(t, workers, 5)

	for i := 1; i < len(workers); i++ {
		p, err := os.FindProcess(int(workers[i].Pid))
		require.NoError(t, err)
		require.NoError(t, p.Kill())
	}

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	// activity can't complete at this moment
	time.Sleep(time.Second)

	// restore the script and recover activity pool
	_ = os.Rename("../php_test_files/worker.bak", "../php_test_files/worker.php")

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "HELLO WORLD", result)
	stopCh <- struct{}{}
	wg.Wait()
}

// ----- LA

func Test_WorkerError_DisasterRecovery_HeavyLA(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto-la.yaml")

	defer func() {
		// always restore script
		_ = os.Rename("../php_test_files/worker-la.bak", "../php_test_files/worker-la.php")
	}()

	// Makes worker pool unable to recover for some time
	require.NoError(t, os.Rename("../php_test_files/worker-la.php", "../php_test_files/worker-la.bak"))

	list, err := helpers.Workers(t.Context())
	require.NoError(t, err)

	p, err := os.FindProcess(int(list[0].GetPid()))
	assert.NoError(t, err)

	// must fully recover with new worker
	assert.NoError(t, p.Kill())

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	time.Sleep(time.Second * 5)

	// restore the script and recover activity pool
	_ = os.Rename("../php_test_files/worker-la.bak", "../php_test_files/worker-la.php")

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_WorkerErrorLA_DisasterRecovery(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto-la.yaml")

	workers := getWorkers(t)
	require.Len(t, workers, 5)

	p, err := os.FindProcess(int(workers[0].Pid))
	assert.NoError(t, err)

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	time.Sleep(time.Second)

	// must fully recover with new worker
	assert.NoError(t, p.Kill())

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ResetLAAll(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)
	time.Sleep(time.Millisecond * 750)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)

	reset(t)

	w, err = s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)
	time.Sleep(time.Millisecond * 750)

	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)

	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ActivityErrorLA_DisasterRecovery(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	defer func() {
		// always restore script
		_ = os.Rename("../php_test_files/worker-la.bak", "../php_test_files/worker-la.php")
	}()

	// Makes worker pool unable to recover for some time
	_ = os.Rename("../php_test_files/worker-la.php", "../php_test_files/worker-la.bak")

	// destroys all workers in activities

	workers := getWorkers(t)
	require.Len(t, workers, 5)

	for i := 1; i < len(workers); i++ {
		p, err := os.FindProcess(int(workers[i].Pid))
		require.NoError(t, err)
		require.NoError(t, p.Kill())
	}

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	// activity can't complete at this moment
	time.Sleep(time.Second)

	// restore the script and recover activity pool
	_ = os.Rename("../php_test_files/worker-la.bak", "../php_test_files/worker-la.php")

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "HELLO WORLD", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_WorkerErrorLA_DisasterRecoveryProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	workers := getWorkers(t)
	require.Len(t, workers, 5)

	p, err := os.FindProcess(int(workers[0].Pid))
	assert.NoError(t, err)

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	time.Sleep(time.Second)

	// must fully recover with new worker
	assert.NoError(t, p.Kill())

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ActivityErrorLA_DisasterRecoveryProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	defer func() {
		// always restore script
		_ = os.Rename("../php_test_files/worker-la.bak", "../php_test_files/worker-la.php")
	}()

	// Makes worker pool unable to recover for some time
	_ = os.Rename("../php_test_files/worker-la.php", "../php_test_files/worker-la.bak")

	// destroys all workers in activities
	workers := getWorkers(t)
	require.Len(t, workers, 5)

	for i := 1; i < len(workers); i++ {
		p, err := os.FindProcess(int(workers[i].Pid))
		require.NoError(t, err)
		require.NoError(t, p.Kill())
	}

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	// activity can't complete at this moment
	time.Sleep(time.Millisecond * 750)

	// restore the script and recover activity pool
	_ = os.Rename("../php_test_files/worker-la.bak", "../php_test_files/worker-la.php")

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "HELLO WORLD", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func getWorkers(t *testing.T) []*informerV1.ProcessState {
	list, err := helpers.Workers(t.Context())
	assert.NoError(t, err)
	assert.Len(t, list, 5)

	return list
}

func reset(t *testing.T) {
	require.NoError(t, helpers.Reset(t.Context()))
}
