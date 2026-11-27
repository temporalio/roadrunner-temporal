package aggregatedpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	enumspb "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"

	"github.com/nexus-rpc/sdk-go/nexus"
)

func handlerFailure(message, errType string, retry enumspb.NexusHandlerErrorRetryBehavior) *failurepb.Failure {
	return &failurepb.Failure{
		Message: message,
		FailureInfo: &failurepb.Failure_NexusHandlerFailureInfo{
			NexusHandlerFailureInfo: &failurepb.NexusHandlerFailureInfo{
				Type:          errType,
				RetryBehavior: retry,
			},
		},
	}
}

func applicationFailure(message, errType string) *failurepb.Failure {
	return &failurepb.Failure{
		Message: message,
		FailureInfo: &failurepb.Failure_ApplicationFailureInfo{
			ApplicationFailureInfo: &failurepb.ApplicationFailureInfo{Type: errType},
		},
	}
}

func TestNexusErrorFromFailure_HandlerErrors(t *testing.T) {
	cases := []struct {
		name  string
		input *failurepb.Failure
		typ   nexus.HandlerErrorType
		retry nexus.HandlerErrorRetryBehavior
	}{
		{
			name:  "non retryable bad request",
			input: handlerFailure("payload parsing failed", "BAD_REQUEST", enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_NON_RETRYABLE),
			typ:   nexus.HandlerErrorTypeBadRequest,
			retry: nexus.HandlerErrorRetryBehaviorNonRetryable,
		},
		{
			name:  "retryable internal",
			input: handlerFailure("try again", "INTERNAL", enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_RETRYABLE),
			typ:   nexus.HandlerErrorTypeInternal,
			retry: nexus.HandlerErrorRetryBehaviorRetryable,
		},
		{
			name:  "unspecified retry behavior",
			input: handlerFailure("unavailable", "UNAVAILABLE", enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_UNSPECIFIED),
			typ:   nexus.HandlerErrorTypeUnavailable,
			retry: nexus.HandlerErrorRetryBehaviorUnspecified,
		},
		{
			name:  "application failure without a nexus type",
			input: applicationFailure("boom", "RuntimeException"),
			typ:   nexus.HandlerErrorTypeInternal,
			retry: nexus.HandlerErrorRetryBehaviorUnspecified,
		},
		{
			name:  "failure without failure info",
			input: &failurepb.Failure{Message: "nothing to go on"},
			typ:   nexus.HandlerErrorTypeInternal,
			retry: nexus.HandlerErrorRetryBehaviorUnspecified,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var he *nexus.HandlerError
			require.ErrorAs(t, nexusErrorFromFailure(tc.input), &he)
			assert.Equal(t, tc.typ, he.Type)
			assert.Equal(t, tc.retry, he.RetryBehavior)
			assert.Equal(t, tc.input.GetMessage(), he.Message)
			assert.ErrorContains(t, he.Cause, tc.input.GetMessage())
		})
	}
}

func TestNexusErrorFromFailure_OperationErrors(t *testing.T) {
	cases := []struct {
		name  string
		input *failurepb.Failure
		state nexus.OperationState
	}{
		{
			name:  "canceled failure info",
			input: &failurepb.Failure{Message: "canceled", FailureInfo: &failurepb.Failure_CanceledFailureInfo{CanceledFailureInfo: &failurepb.CanceledFailureInfo{}}},
			state: nexus.OperationStateCanceled,
		},
		{
			name:  "operation error type",
			input: applicationFailure("deliberate failure", nexusOperationErrorType),
			state: nexus.OperationStateFailed,
		},
		{
			name:  "legacy prefixed failed state",
			input: applicationFailure("deliberate failure", nexusOperationErrorTypePrefix+"failed"),
			state: nexus.OperationStateFailed,
		},
		{
			name:  "legacy prefixed canceled state",
			input: applicationFailure("stopped", nexusOperationErrorTypePrefix+"canceled"),
			state: nexus.OperationStateCanceled,
		},
		{
			name:  "legacy prefixed unknown state coerces to failed",
			input: applicationFailure("huh", nexusOperationErrorTypePrefix+"running"),
			state: nexus.OperationStateFailed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var oe *nexus.OperationError
			require.ErrorAs(t, nexusErrorFromFailure(tc.input), &oe)
			assert.Equal(t, tc.state, oe.State)
			assert.Equal(t, tc.input.GetMessage(), oe.Message)
			assert.ErrorContains(t, oe.Cause, tc.input.GetMessage())
		})
	}
}
