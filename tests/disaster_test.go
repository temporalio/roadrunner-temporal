package tests

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/client"
)

func Test_WorkerError_DisasterRecovery(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	p, err := os.FindProcess(s.workflows.Workers()[0].Pid)
	assert.NoError(t, err)

	w, err := s.Client().ExecuteWorkflow(
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

// TODO find a solution
// func Test_WorkerError_DisasterRecovery_Heavy(t *testing.T) {
//	stopCh := make(chan struct{}, 1)
//	wg := &sync.WaitGroup{}
//	wg.Add(1)
//	s := NewTestServer(t, stopCh, wg, false)
//
//	defer func() {
//		// always restore script
//		_ = os.Rename("worker.bak", "worker.php")
//	}()
//
//	// Makes worker pool unable to recover for some time
//	_ = os.Rename("worker.php", "worker.bak")
//
//	p, err := os.FindProcess(int(s.workflows.Workers()[0].Pid()))
//	assert.NoError(t, err)
//
//	// must fully recover with new worker
//	assert.NoError(t, p.Kill())
//
//	w, err := s.Client().ExecuteWorkflow(
//		context.Background(),
//		client.StartWorkflowOptions{
//			TaskQueue: "default",
//		},
//		"TimerWorkflow",
//		"Hello World",
//	)
//	assert.NoError(t, err)
//
//	time.Sleep(time.Millisecond * 750)
//
//	// restore the script and recover activity pool
//	_ = os.Rename("worker.bak", "worker.php")
//
//	var result string
//	assert.NoError(t, w.Get(context.Background(), &result))
//	assert.Equal(t, "hello world", result)
//	stopCh <- struct{}{}
//	wg.Wait()
// }

func Test_ActivityError_DisasterRecovery(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	defer func() {
		// always restore script
		_ = os.Rename("worker.bak", "worker.php")
	}()

	// Makes worker pool unable to recover for some time
	_ = os.Rename("worker.php", "worker.bak")

	// destroys all workers in activities
	for _, wrk := range s.activities.BaseProcesses() {
		assert.NoError(t, wrk.Kill())
	}

	w, err := s.Client().ExecuteWorkflow(
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
	_ = os.Rename("worker.bak", "worker.php")

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
	s := NewTestServer(t, stopCh, wg, true)

	p, err := os.FindProcess(s.workflows.Workers()[0].Pid)
	assert.NoError(t, err)

	w, err := s.Client().ExecuteWorkflow(
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

// TODO find a solution
// func Test_WorkerError_DisasterRecovery_HeavyProto(t *testing.T) {
//	stopCh := make(chan struct{}, 1)
//	wg := &sync.WaitGroup{}
//	wg.Add(1)
//	s := NewTestServer(t, stopCh, wg, true)
//
//	defer func() {
//		// always restore script
//		_ = os.Rename("worker.bak", "worker.php")
//	}()
//
//	// Makes worker pool unable to recover for some time
//	_ = os.Rename("worker.php", "worker.bak")
//
//	p, err := os.FindProcess(int(s.workflows.Workers()[0].Pid()))
//	assert.NoError(t, err)
//
//	// must fully recover with new worker
//	assert.NoError(t, p.Kill())
//
//	w, err := s.Client().ExecuteWorkflow(
//		context.Background(),
//		client.StartWorkflowOptions{
//			TaskQueue: "default",
//		},
//		"TimerWorkflow",
//		"Hello World",
//	)
//	assert.NoError(t, err)
//
//	time.Sleep(time.Millisecond * 750)
//
//	// restore the script and recover activity pool
//	_ = os.Rename("worker.bak", "worker.php")
//
//	var result string
//	assert.NoError(t, w.Get(context.Background(), &result))
//	assert.Equal(t, "hello world", result)
//	stopCh <- struct{}{}
//	wg.Wait()
// }

func Test_ActivityError_DisasterRecoveryProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	defer func() {
		// always restore script
		_ = os.Rename("worker.bak", "worker.php")
	}()

	// Makes worker pool unable to recover for some time
	_ = os.Rename("worker.php", "worker.bak")

	// destroys all workers in activities
	for _, wrk := range s.activities.BaseProcesses() {
		assert.NoError(t, wrk.Kill())
	}

	w, err := s.Client().ExecuteWorkflow(
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
	_ = os.Rename("worker.bak", "worker.php")

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "HELLO WORLD", result)
	stopCh <- struct{}{}
	wg.Wait()
}
