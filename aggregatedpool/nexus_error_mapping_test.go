package aggregatedpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	enumspb "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"

	nexus "github.com/nexus-rpc/sdk-go/nexus"
)

// ── Failure → Nexus error mapping ──────────────────────────────────

func TestNexusErrorFromFailure_HandlerFailureInfoPreservesType(t *testing.T) {
	f := &failurepb.Failure{
		Message: "payload parsing failed",
		FailureInfo: &failurepb.Failure_NexusHandlerFailureInfo{
			NexusHandlerFailureInfo: &failurepb.NexusHandlerFailureInfo{
				Type:          "BAD_REQUEST",
				RetryBehavior: enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_NON_RETRYABLE,
			},
		},
	}

	err := nexusErrorFromFailure(f)

	he, ok := err.(*nexus.HandlerError)
	require.True(t, ok, "expected *nexus.HandlerError, got %T", err)
	assert.Equal(t, nexus.HandlerErrorTypeBadRequest, he.Type)
	assert.Equal(t, nexus.HandlerErrorRetryBehaviorNonRetryable, he.RetryBehavior)
	assert.Equal(t, "payload parsing failed", he.Message)
}

func TestNexusErrorFromFailure_RetryBehaviorRetryable(t *testing.T) {
	f := &failurepb.Failure{
		Message: "try again",
		FailureInfo: &failurepb.Failure_NexusHandlerFailureInfo{
			NexusHandlerFailureInfo: &failurepb.NexusHandlerFailureInfo{
				Type:          "INTERNAL",
				RetryBehavior: enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_RETRYABLE,
			},
		},
	}

	err := nexusErrorFromFailure(f)
	he := err.(*nexus.HandlerError)
	assert.Equal(t, nexus.HandlerErrorTypeInternal, he.Type)
	assert.Equal(t, nexus.HandlerErrorRetryBehaviorRetryable, he.RetryBehavior)
}

func TestNexusErrorFromFailure_RetryBehaviorUnspecifiedDefaults(t *testing.T) {
	f := &failurepb.Failure{
		Message: "",
		FailureInfo: &failurepb.Failure_NexusHandlerFailureInfo{
			NexusHandlerFailureInfo: &failurepb.NexusHandlerFailureInfo{
				Type: "NOT_FOUND",
				// RetryBehavior left zero-value
			},
		},
	}

	he := nexusErrorFromFailure(f).(*nexus.HandlerError)
	assert.Equal(t, nexus.HandlerErrorTypeNotFound, he.Type)
	assert.Equal(t, nexus.HandlerErrorRetryBehaviorUnspecified, he.RetryBehavior)
}

func TestNexusErrorFromFailure_AllSpecErrorTypesRoundTrip(t *testing.T) {
	// Every entry from nexus-rpc/api/SPEC.md#predefined-handler-errors.
	cases := []struct {
		wire string
		want nexus.HandlerErrorType
	}{
		{"BAD_REQUEST", nexus.HandlerErrorTypeBadRequest},
		{"UNAUTHENTICATED", nexus.HandlerErrorTypeUnauthenticated},
		{"UNAUTHORIZED", nexus.HandlerErrorTypeUnauthorized},
		{"NOT_FOUND", nexus.HandlerErrorTypeNotFound},
		{"REQUEST_TIMEOUT", nexus.HandlerErrorTypeRequestTimeout},
		{"CONFLICT", nexus.HandlerErrorTypeConflict},
		{"RESOURCE_EXHAUSTED", nexus.HandlerErrorTypeResourceExhausted},
		{"INTERNAL", nexus.HandlerErrorTypeInternal},
		{"NOT_IMPLEMENTED", nexus.HandlerErrorTypeNotImplemented},
		{"UNAVAILABLE", nexus.HandlerErrorTypeUnavailable},
		{"UPSTREAM_TIMEOUT", nexus.HandlerErrorTypeUpstreamTimeout},
	}

	for _, c := range cases {
		t.Run(c.wire, func(t *testing.T) {
			f := &failurepb.Failure{
				Message: c.wire,
				FailureInfo: &failurepb.Failure_NexusHandlerFailureInfo{
					NexusHandlerFailureInfo: &failurepb.NexusHandlerFailureInfo{
						Type: c.wire,
					},
				},
			}
			he := nexusErrorFromFailure(f).(*nexus.HandlerError)
			assert.Equal(t, c.want, he.Type)
		})
	}
}

func TestNexusErrorFromFailure_OperationErrorFailed(t *testing.T) {
	f := &failurepb.Failure{
		Message: "user rejected",
		FailureInfo: &failurepb.Failure_ApplicationFailureInfo{
			ApplicationFailureInfo: &failurepb.ApplicationFailureInfo{
				Type: "nexus.OperationError.failed",
			},
		},
	}

	err := nexusErrorFromFailure(f)
	oe, ok := err.(*nexus.OperationError)
	require.True(t, ok, "expected *nexus.OperationError, got %T", err)
	assert.Equal(t, nexus.OperationStateFailed, oe.State)
}

func TestNexusErrorFromFailure_OperationErrorCanceled(t *testing.T) {
	f := &failurepb.Failure{
		Message: "user canceled",
		FailureInfo: &failurepb.Failure_ApplicationFailureInfo{
			ApplicationFailureInfo: &failurepb.ApplicationFailureInfo{
				Type: "nexus.OperationError.canceled",
			},
		},
	}

	oe := nexusErrorFromFailure(f).(*nexus.OperationError)
	assert.Equal(t, nexus.OperationStateCanceled, oe.State)
}

func TestNexusErrorFromFailure_OperationErrorUnknownStateFallsBackToFailed(t *testing.T) {
	f := &failurepb.Failure{
		Message: "weird",
		FailureInfo: &failurepb.Failure_ApplicationFailureInfo{
			ApplicationFailureInfo: &failurepb.ApplicationFailureInfo{
				Type: "nexus.OperationError.weird",
			},
		},
	}

	oe := nexusErrorFromFailure(f).(*nexus.OperationError)
	assert.Equal(t, nexus.OperationStateFailed, oe.State, "unknown state must not leak to the wire")
}

func TestNexusErrorFromFailure_UntaggedApplicationFailureFallsBackToInternal(t *testing.T) {
	f := &failurepb.Failure{
		Message: "boom",
		FailureInfo: &failurepb.Failure_ApplicationFailureInfo{
			ApplicationFailureInfo: &failurepb.ApplicationFailureInfo{
				Type: "SomeUserType",
			},
		},
	}

	he, ok := nexusErrorFromFailure(f).(*nexus.HandlerError)
	require.True(t, ok)
	assert.Equal(t, nexus.HandlerErrorTypeInternal, he.Type, "unknown failure shape must collapse to Internal")
	assert.Equal(t, "boom", he.Message)
}

func TestNexusErrorFromFailure_NoFailureInfoIsInternal(t *testing.T) {
	f := &failurepb.Failure{Message: "bare failure"}

	he, ok := nexusErrorFromFailure(f).(*nexus.HandlerError)
	require.True(t, ok)
	assert.Equal(t, nexus.HandlerErrorTypeInternal, he.Type)
	assert.Equal(t, "bare failure", he.Message)
}

// ── RetryBehavior enum mapping ──────────────────────────────────────

func TestMapNexusRetryBehavior_AllValues(t *testing.T) {
	cases := []struct {
		in   enumspb.NexusHandlerErrorRetryBehavior
		want nexus.HandlerErrorRetryBehavior
	}{
		{enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_UNSPECIFIED, nexus.HandlerErrorRetryBehaviorUnspecified},
		{enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_RETRYABLE, nexus.HandlerErrorRetryBehaviorRetryable},
		{enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_NON_RETRYABLE, nexus.HandlerErrorRetryBehaviorNonRetryable},
	}

	for _, c := range cases {
		t.Run(c.in.String(), func(t *testing.T) {
			got := mapNexusRetryBehavior(c.in)
			assert.Equal(t, c.want, got)
		})
	}
}
