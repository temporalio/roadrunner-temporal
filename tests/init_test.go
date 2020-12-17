package tests

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_InitWorkers(t *testing.T) {
	s := NewTestServer()
	defer s.MustClose()

	assert.Contains(t, s.workflows.WorkflowNames(), "DelayedWorkflow")
}
