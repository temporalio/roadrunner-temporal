package tests

import (
	"bytes"
	"context"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
)

func Test_OtlpInterceptor(t *testing.T) {
	rd, wr, err := os.Pipe()
	assert.NoError(t, err)
	os.Stderr = wr

	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)

	s := NewTestServerWithOtelInterceptor(t, stopCh, wg)

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleWorkflow",
		"test-input",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "TEST-INPUT", result)

	we, err := s.Client.DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	assert.Equal(t, "Completed", we.WorkflowExecutionInfo.Status.String())

	stopCh <- struct{}{}
	wg.Wait()

	time.Sleep(time.Second)
	_ = wr.Close()
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, rd)
	require.NoError(t, err)

	// contains spans
	require.Contains(t, buf.String(), `"Name": "RunActivity:SimpleActivity.echo",`)
}
