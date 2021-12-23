package tests

import (
	"context"
	"sync"
	"testing"

	"github.com/spiral/sdk-go/client"
	"github.com/stretchr/testify/assert"
)

func Test_ExecuteChildWorkflowProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"WithChildWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "Child: CHILD HELLO WORLD", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ExecuteChildStubWorkflowProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
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
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ExecuteChildStubWorkflow_02Proto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"ChildStubWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result []string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, []string{"HELLO WORLD", "UNTYPED"}, result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_SignalChildViaStubWorkflowProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SignalChildViaStubWorkflow",
	)
	assert.NoError(t, err)

	var result int
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, 8, result)
	stopCh <- struct{}{}
	wg.Wait()
}
