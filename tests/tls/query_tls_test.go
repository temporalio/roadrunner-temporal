package tls

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
	"go.temporal.io/sdk/worker"
)

func Test_ListQueriesProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServerTLS(t, stopCh, wg, ".rr-proto.yaml")

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

	v, err := s.Client.QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "error", -1)
	assert.Nil(t, v)
	assert.Error(t, err)

	assert.Contains(t, err.Error(), "KnownQueryTypes=[get]")

	var r int
	assert.NoError(t, w.Get(context.Background(), &r))
	assert.Equal(t, 0, r)

	worker.PurgeStickyWorkflowCache()

	time.Sleep(time.Second)

	workers := getWorkers(t)

	for i := 0; i < len(workers); i++ {
		proc, errF := os.FindProcess(int(workers[i].Pid))
		require.NoError(t, errF)
		_ = proc.Kill()
	}

	// Poll until workers recover and the query returns the expected error.
	// On slow CI runners, worker recovery can take over 60 seconds.
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		qCtx, qCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer qCancel()
		qv, qerr := s.Client.QueryWorkflow(qCtx, w.GetID(), w.GetRunID(), "error", -1)
		assert.Nil(collect, qv)
		assert.Error(collect, qerr)
		if qerr != nil {
			assert.Contains(collect, qerr.Error(), "KnownQueryTypes=[get]")
		}
	}, 2*time.Minute, 3*time.Second)

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
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
	s := helpers.NewTestServerTLS(t, stopCh, wg, ".rr-proto.yaml")

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
	s := helpers.NewTestServerTLS(t, stopCh, wg, ".rr-proto-la.yaml")

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
	s := helpers.NewTestServerTLS(t, stopCh, wg, ".rr-proto-la.yaml")

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
