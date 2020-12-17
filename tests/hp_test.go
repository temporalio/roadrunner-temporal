package tests

import (
	"context"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/client"
	"testing"
)

func Test_VerifyRegistration(t *testing.T) {
	s := NewTestServer()
	defer s.MustClose()

	assert.Contains(t, s.workflows.WorkflowNames(), "SimpleWorkflow")
	assert.Contains(t, s.activities.ActivityNames(), "SimpleActivity.echo")

	// todo: fix bug
	//assert.Contains(t, s.activities.ActivityNames(), "SimpleActivity.lower")
}

func Test_ExecuteSimpleWorkflow(t *testing.T) {
	s := NewTestServer()
	defer s.MustClose()

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "HELLO WORLD", result)
}
