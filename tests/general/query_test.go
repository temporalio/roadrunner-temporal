package tests

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"tests/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
)

func Test_ListQueriesProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"QueryWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	time.Sleep(time.Second * 2)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	v, err := s.Client.QueryWorkflow(ctx, w.GetID(), w.GetRunID(), "error", -1)
	assert.Nil(t, v)
	assert.Error(t, err)
	cancel()

	assert.Contains(t, err.Error(), "KnownQueryTypes=[get]")

	var r int
	ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
	assert.NoError(t, w.Get(ctx, &r))
	assert.Equal(t, 0, r)
	cancel()

	time.Sleep(time.Millisecond * 500)

	workers := getWorkers(t)

	for i := 0; i < len(workers); i++ {
		proc, errF := os.FindProcess(int(workers[i].Pid))
		require.NoError(t, errF)
		_ = proc.Kill()
	}

	time.Sleep(time.Second * 10)

	ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
	v, err = s.Client.QueryWorkflow(ctx, w.GetID(), w.GetRunID(), "error", -1)
	assert.Nil(t, v)
	assert.Error(t, err)
	time.Sleep(time.Second)
	cancel()

	assert.Contains(t, err.Error(), "KnownQueryTypes=[get]")

	ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
	assert.NoError(t, w.Get(ctx, &r))
	assert.Equal(t, 0, r)
	cancel()

	s.Client.Close()
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_GetQueryProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"QueryWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), "add", 88)
	assert.NoError(t, err)
	time.Sleep(time.Millisecond * 500)

	v, err := s.Client.QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "get", nil)
	assert.NoError(t, err)

	var r int
	assert.NoError(t, v.Get(&r))
	assert.Equal(t, 88, r)

	assert.NoError(t, w.Get(context.Background(), &r))
	assert.Equal(t, 88, r)
	stopCh <- struct{}{}
	wg.Wait()
}

// ----- LA

func Test_ListQueriesLAProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto-la.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"QueryWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	time.Sleep(time.Second)

	v, err := s.Client.QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "error", -1)
	assert.Nil(t, v)
	assert.Error(t, err)

	assert.Contains(t, err.Error(), "KnownQueryTypes=[get]")

	var r int
	assert.NoError(t, w.Get(context.Background(), &r))
	assert.Equal(t, 0, r)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_GetQueryLAProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto-la.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"QueryWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), "add", 88)
	assert.NoError(t, err)
	time.Sleep(time.Millisecond * 500)

	v, err := s.Client.QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "get", nil)
	assert.NoError(t, err)

	var r int
	assert.NoError(t, v.Get(&r))
	assert.Equal(t, 88, r)

	assert.NoError(t, w.Get(context.Background(), &r))
	assert.Equal(t, 88, r)
	stopCh <- struct{}{}
	wg.Wait()
}
