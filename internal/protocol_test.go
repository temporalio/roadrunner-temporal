package internal

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type stubWorkflowEnvironment struct {
	bindings.WorkflowEnvironment

	info             *workflow.Info
	dataConverter    converter.DataConverter
	failureConverter converter.FailureConverter
}

func (e *stubWorkflowEnvironment) WorkflowInfo() *workflow.Info { return e.info }
func (e *stubWorkflowEnvironment) GetDataConverter() converter.DataConverter {
	return e.dataConverter
}
func (e *stubWorkflowEnvironment) GetFailureConverter() converter.FailureConverter {
	return e.failureConverter
}

func TestLocalActivityParams_FailureConverterDoesNotPanic(t *testing.T) {
	env := &stubWorkflowEnvironment{
		info:             &workflow.Info{TaskQueueName: "tq"},
		dataConverter:    converter.GetDefaultDataConverter(),
		failureConverter: temporal.GetDefaultFailureConverter(),
	}
	params := ExecuteLocalActivity{Name: "X"}.LocalActivityParams(
		env, func() {}, &commonpb.Payloads{}, &commonpb.Header{},
	)

	require.NotPanics(t, func() {
		_ = params.FailureConverter.ErrorToFailure(errors.New("boom"))
	})
}
