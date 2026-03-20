package tests

import (
	"context"
	"sync"
	"testing"
	"tests/helpers"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.temporal.io/sdk/client"
)

func Test_OtlpInterceptor(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)

	s, otelInterceptor := helpers.NewTestServerWithOtelInterceptor(t, stopCh, wg)

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

	// Allow spans to be flushed
	time.Sleep(time.Second)

	spans := otelInterceptor.Exp.GetSpans()
	require.NotEmpty(t, spans, "expected OTEL spans to be captured")

	var found bool
	for _, span := range spans {
		if span.Name == "RunActivity:SimpleActivity.echo" {
			found = true
			break
		}
	}
	require.True(t, found, "expected span RunActivity:SimpleActivity.echo, got spans: %v", spanNames(spans))
}

func spanNames(spans tracetest.SpanStubs) []string {
	names := make([]string, len(spans))
	for i, s := range spans {
		names[i] = s.Name
	}
	return names
}
