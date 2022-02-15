package tests

import (
	"context"
	"net"
	"net/rpc"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/roadrunner-server/api/v2/state/process"
	goridgeRpc "github.com/roadrunner-server/goridge/v3/pkg/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
)

func Test_WorkerError_DisasterRecovery(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg)

	workers := getWorkers(t)
	require.Len(t, workers, 5)

	p, err := os.FindProcess(workers[0].Pid)
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

func Test_ResetAll(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg)

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

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)

	reset(t)

	w, err = s.Client().ExecuteWorkflow(
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

// TODO find a solution
// func Test_WorkerError_DisasterRecovery_Heavy(t *testing.T) {
//	stopCh := make(chan struct{}, 1)
//	wg := &sync.WaitGroup{}
//	wg.Add(1)
//	s := NewTestServer(t, stopCh, wg)
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
	s := NewTestServer(t, stopCh, wg)

	defer func() {
		// always restore script
		_ = os.Rename("worker.bak", "worker.php")
	}()

	// Makes worker pool unable to recover for some time
	_ = os.Rename("worker.php", "worker.bak")

	// destroys all workers in activities

	workers := getWorkers(t)
	require.Len(t, workers, 5)

	for i := 1; i < len(workers); i++ {
		p, err := os.FindProcess(workers[i].Pid)
		require.NoError(t, err)
		require.NoError(t, p.Kill())
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
	s := NewTestServer(t, stopCh, wg)

	workers := getWorkers(t)
	require.Len(t, workers, 5)

	p, err := os.FindProcess(workers[0].Pid)
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
//	s := NewTestServer(t, stopCh, wg)
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
	s := NewTestServer(t, stopCh, wg)

	defer func() {
		// always restore script
		_ = os.Rename("worker.bak", "worker.php")
	}()

	// Makes worker pool unable to recover for some time
	_ = os.Rename("worker.php", "worker.bak")

	// destroys all workers in activities
	workers := getWorkers(t)
	require.Len(t, workers, 5)

	for i := 1; i < len(workers); i++ {
		p, err := os.FindProcess(workers[i].Pid)
		require.NoError(t, err)
		require.NoError(t, p.Kill())
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

func getWorkers(t *testing.T) []*process.State {
	conn, err := net.Dial("tcp", "127.0.0.1:6001")
	assert.NoError(t, err)
	c := rpc.NewClientWithCodec(goridgeRpc.NewClientCodec(conn))
	// WorkerList contains list of workers.
	list := struct {
		// Workers is list of workers.
		Workers []*process.State `json:"workers"`
	}{}

	err = c.Call("informer.Workers", "temporal", &list)
	assert.NoError(t, err)
	assert.Len(t, list.Workers, 5)

	return list.Workers
}

func reset(t *testing.T) {
	conn, err := net.Dial("tcp", "127.0.0.1:6001")
	assert.NoError(t, err)
	c := rpc.NewClientWithCodec(goridgeRpc.NewClientCodec(conn))

	var ret bool
	err = c.Call("resetter.Reset", "temporal", &ret)
	assert.NoError(t, err)
	require.True(t, ret)
}
