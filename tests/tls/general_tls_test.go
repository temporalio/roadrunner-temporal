package tls

import (
	"context"
	"sync"
	"testing"
	"time"

	"tests"

	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/client"
)

func Test_HistoryLen(t *testing.T) {
	t.Skip("skipping test")
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)

	s := tests.NewTestServerTLS(t, stopCh, wg, ".rr-proto.yaml")
	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"HistoryLengthWorkflow")
	assert.NoError(t, err)

	time.Sleep(time.Second)
	var result any

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	assert.NoError(t, w.Get(ctx, &result))

	res := []float64{3, 8, 8, 15}
	out := result.([]interface{})

	for i := 0; i < len(res); i++ {
		if res[i] != out[i].(float64) {
			t.Fail()
		}
	}

	we, err := s.Client.DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	assert.Equal(t, "Completed", we.WorkflowExecutionInfo.Status.String())
	stopCh <- struct{}{}
	wg.Wait()
	time.Sleep(time.Second)
}
