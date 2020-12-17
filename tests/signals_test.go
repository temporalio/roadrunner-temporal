package tests

import (
	"context"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/client"
	"testing"
)

func Test_ExecuteSimpleSignalledWorkflow_noSignals(t *testing.T) {
	s := NewTestServer()
	defer s.MustClose()

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleSignalledWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result int
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, 0, result)
}
