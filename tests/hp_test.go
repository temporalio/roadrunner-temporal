package tests

import (
	"github.com/stretchr/testify/assert"
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
