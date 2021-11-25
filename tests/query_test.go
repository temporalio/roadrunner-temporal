package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/spiral/sdk-go/client"
	"github.com/stretchr/testify/assert"
)

func Test_ListQueries(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"QueryWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	time.Sleep(time.Millisecond * 500)

	v, err := s.Client().QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "error", -1)
	assert.Nil(t, v)
	assert.Error(t, err)

	assert.Contains(t, err.Error(), "KnownQueryTypes=[get]")

	var r int
	assert.NoError(t, w.Get(context.Background(), &r))
	assert.Equal(t, 0, r)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_GetQuery(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"QueryWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	err = s.Client().SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), "add", 88)
	assert.NoError(t, err)
	time.Sleep(time.Millisecond * 500)

	v, err := s.Client().QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "get", nil)
	assert.NoError(t, err)

	var r int
	assert.NoError(t, v.Get(&r))
	assert.Equal(t, 88, r)

	assert.NoError(t, w.Get(context.Background(), &r))
	assert.Equal(t, 88, r)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ListQueriesProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"QueryWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	time.Sleep(time.Millisecond * 500)

	v, err := s.Client().QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "error", -1)
	assert.Nil(t, v)
	assert.Error(t, err)

	assert.Contains(t, err.Error(), "KnownQueryTypes=[get]")

	var r int
	assert.NoError(t, w.Get(context.Background(), &r))
	assert.Equal(t, 0, r)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_GetQueryProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"QueryWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	err = s.Client().SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), "add", 88)
	assert.NoError(t, err)
	time.Sleep(time.Millisecond * 500)

	v, err := s.Client().QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "get", nil)
	assert.NoError(t, err)

	var r int
	assert.NoError(t, v.Get(&r))
	assert.Equal(t, 88, r)

	assert.NoError(t, w.Get(context.Background(), &r))
	assert.Equal(t, 88, r)
	stopCh <- struct{}{}
	wg.Wait()
}
