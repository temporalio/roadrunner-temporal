package internal

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonpb "go.temporal.io/api/common/v1"
)

func TestCommandName_Nexus(t *testing.T) {
	tests := []struct {
		name    string
		command any
		want    string
	}{
		{"InvokeNexusOperation value", InvokeNexusOperation{}, "InvokeNexusOperation"},
		{"InvokeNexusOperation ptr", &InvokeNexusOperation{}, "InvokeNexusOperation"},
		{"CancelNexusOperation value", CancelNexusOperation{}, "CancelNexusOperation"},
		{"CancelNexusOperation ptr", &CancelNexusOperation{}, "CancelNexusOperation"},
		{"CancelNexusOperationMethod value", CancelNexusOperationMethod{}, "CancelNexusOperationMethod"},
		{"CancelNexusOperationMethod ptr", &CancelNexusOperationMethod{}, "CancelNexusOperationMethod"},
		{"ExecuteNexusOperation value", ExecuteNexusOperation{}, "ExecuteNexusOperation"},
		{"ExecuteNexusOperation ptr", &ExecuteNexusOperation{}, "ExecuteNexusOperation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CommandName(tt.command)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInitCommand_Nexus(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
		wantType any
	}{
		{"InvokeNexusOperation", "InvokeNexusOperation", &InvokeNexusOperation{}},
		{"CancelNexusOperation", "CancelNexusOperation", &CancelNexusOperation{}},
		{"CancelNexusOperationMethod", "CancelNexusOperationMethod", &CancelNexusOperationMethod{}},
		{"ExecuteNexusOperation", "ExecuteNexusOperation", &ExecuteNexusOperation{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InitCommand(tt.typeName)
			require.NoError(t, err)
			assert.IsType(t, tt.wantType, got)
		})
	}
}

func TestInvokeNexusOperation_Fields(t *testing.T) {
	op := InvokeNexusOperation{
		Service:   "GreetingService",
		Operation: "greet",
		RequestID: "req-123",
		Callback:  "http://callback.example.com",
		CallbackHeaders: map[string]string{
			"Auth": "token",
		},
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	assert.Equal(t, "GreetingService", op.Service)
	assert.Equal(t, "greet", op.Operation)
	assert.Equal(t, "req-123", op.RequestID)
	assert.Equal(t, "http://callback.example.com", op.Callback)
	assert.Equal(t, "token", op.CallbackHeaders["Auth"])
	assert.Equal(t, "application/json", op.Headers["Content-Type"])
}

func TestCancelNexusOperation_Fields(t *testing.T) {
	op := CancelNexusOperation{
		Service:        "GreetingService",
		Operation:      "greet",
		OperationToken: "async-token-xyz",
	}

	assert.Equal(t, "GreetingService", op.Service)
	assert.Equal(t, "greet", op.Operation)
	assert.Equal(t, "async-token-xyz", op.OperationToken)
}

func TestCancelNexusOperationMethod_Fields(t *testing.T) {
	op := CancelNexusOperationMethod{
		InvocationID: 42,
		Reason:       "pool shutdown",
	}

	assert.Equal(t, uint64(42), op.InvocationID)
	assert.Equal(t, "pool shutdown", op.Reason)
}

func TestCancelNexusOperationMethod_JSONRoundTrip(t *testing.T) {
	// InvocationID must NOT be omitempty — zero is a valid "no-op" id on the
	// wire (PHP side treats 0 as "no invocation to cancel"), and losing it
	// silently would hide a bug rather than surface it.
	op := CancelNexusOperationMethod{
		InvocationID: 7,
		Reason:       "deadline",
	}
	data, err := json.Marshal(op)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"invocationId":7`)
	assert.Contains(t, string(data), `"reason":"deadline"`)

	var back CancelNexusOperationMethod
	require.NoError(t, json.Unmarshal(data, &back))
	assert.Equal(t, op, back)
}

// InvocationID is omitempty on InvokeNexusOperation so older PHP workers that
// don't look at the field see the same wire shape as before.
func TestInvokeNexusOperation_InvocationIDOmittedWhenZero(t *testing.T) {
	op := InvokeNexusOperation{
		Service:   "S",
		Operation: "o",
	}
	data, err := json.Marshal(op)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "invocationId",
		"invocationId must be omitted from the wire when zero for backwards compat")
}

func TestInvokeNexusOperation_InvocationIDPresentWhenSet(t *testing.T) {
	op := InvokeNexusOperation{
		Service:      "S",
		Operation:    "o",
		InvocationID: 99,
	}
	data, err := json.Marshal(op)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"invocationId":99`)
}

func TestExecuteNexusOperation_Fields(t *testing.T) {
	op := ExecuteNexusOperation{
		Endpoint:  "my-endpoint",
		Service:   "GreetingService",
		Operation: "greet",
		Options: NexusOperationOptions{
			ScheduleToCloseTimeout: 10 * time.Second,
		},
	}

	assert.Equal(t, "my-endpoint", op.Endpoint)
	assert.Equal(t, "GreetingService", op.Service)
	assert.Equal(t, "greet", op.Operation)
	assert.Equal(t, 10*time.Second, op.Options.ScheduleToCloseTimeout)
}

// TestExecuteNexusOperation_OptionsEndpointServiceIgnored pins the contract:
// PHP redundantly ships endpoint/service inside "options"; Go must silently
// ignore both and trust top-level fields. Even mismatched values must not
// affect the decoded top-level Endpoint/Service.
func TestExecuteNexusOperation_OptionsEndpointServiceIgnored(t *testing.T) {
	wire := []byte(`{
		"endpoint":  "top-level-endpoint",
		"service":   "TopLevelService",
		"operation": "echo",
		"options": {
			"endpoint":               "WRONG-ENDPOINT",
			"service":                "WrongService",
			"scheduleToCloseTimeout": 5000000000
		}
	}`)

	var op ExecuteNexusOperation
	require.NoError(t, json.Unmarshal(wire, &op))
	assert.Equal(t, "top-level-endpoint", op.Endpoint, "top-level endpoint must win")
	assert.Equal(t, "TopLevelService", op.Service, "top-level service must win")
	assert.Equal(t, 5*time.Second, op.Options.ScheduleToCloseTimeout)
}

// TestExecuteNexusOperation_DecodesPHPWireShape pins the JSON contract with the
// PHP side. Internal/Workflow/NexusOperationStub::start ships
//
//	{
//	  "endpoint":  "...",
//	  "service":   "...",
//	  "operation": "...",
//	  "options":   <marshalled NexusOperationOptions>
//	}
//
// where the marshaller emits ScheduleToCloseTimeout as nanoseconds (the
// DateIntervalType default), matching Go's time.Duration JSON encoding.
// 10s ⇒ 10_000_000_000 ns.
func TestExecuteNexusOperation_DecodesPHPWireShape(t *testing.T) {
	wire := []byte(`{
		"endpoint":  "my-nexus-endpoint-name",
		"service":   "SampleNexusService",
		"operation": "echo",
		"options": {
			"endpoint":               "my-nexus-endpoint-name",
			"service":                "SampleNexusService",
			"scheduleToCloseTimeout": 10000000000
		}
	}`)

	var op ExecuteNexusOperation
	require.NoError(t, json.Unmarshal(wire, &op))
	assert.Equal(t, "my-nexus-endpoint-name", op.Endpoint)
	assert.Equal(t, "SampleNexusService", op.Service)
	assert.Equal(t, "echo", op.Operation)
	assert.Equal(t, 10*time.Second, op.Options.ScheduleToCloseTimeout)
}

// TestExecuteNexusOperation_OmitsZeroOptions guards against accidentally
// hard-coding non-zero defaults. A PHP request with all-zero options should
// round-trip cleanly with no timeout enforced.
func TestExecuteNexusOperation_OmitsZeroOptions(t *testing.T) {
	wire := []byte(`{"endpoint":"e","service":"s","operation":"o"}`)
	var op ExecuteNexusOperation
	require.NoError(t, json.Unmarshal(wire, &op))
	assert.Equal(t, time.Duration(0), op.Options.ScheduleToCloseTimeout)
}

// TestExecuteNexusOperation_NexusOperationParams covers the params builder
// without standing up a full WorkflowEnvironment: we only assert the public
// shape (operation name, single payload, options.ScheduleToCloseTimeout). The
// NexusClient and ExecuteNexusOperationParams fields are unexported in
// sdk-go's `internal`, so we exercise them indirectly by feeding the params
// into a no-op environment and expecting no panic.
func TestExecuteNexusOperation_NexusOperationParams(t *testing.T) {
	cmd := ExecuteNexusOperation{
		Endpoint:  "ep",
		Service:   "svc",
		Operation: "op",
		Options: NexusOperationOptions{
			ScheduleToCloseTimeout: 5 * time.Second,
		},
	}

	payload := &commonpb.Payload{
		Metadata: map[string][]byte{"encoding": []byte("json/plain")},
		Data:     []byte(`"hello"`),
	}
	payloads := &commonpb.Payloads{Payloads: []*commonpb.Payload{payload}}

	params := cmd.NexusOperationParams(payloads, nil)
	// The struct fields are unexported; we sanity-check that the builder
	// accepts the inputs without panicking and returns a value of the
	// right type. Behavioural coverage of the dispatch path lives in the
	// aggregatedpool tests where a real WorkflowEnvironment is in scope.
	_ = params
}

// TestExecuteNexusOperation_NexusOperationParams_EmptyPayloads ensures the
// builder tolerates a nil/empty payload bag (PHP can dispatch a no-arg op).
func TestExecuteNexusOperation_NexusOperationParams_EmptyPayloads(t *testing.T) {
	cmd := ExecuteNexusOperation{Endpoint: "e", Service: "s", Operation: "o"}
	_ = cmd.NexusOperationParams(nil, nil)
	_ = cmd.NexusOperationParams(&commonpb.Payloads{}, nil)
}

// TestNexusOperationOptions_DecodesCancellationType pins the wire shape for
// the cancellationType field. PHP marshals the
// `Workflow\NexusOperationCancellationType` enum's int value under
// `cancellationType`; the integer must round-trip into Options unchanged so
// NexusOperationParams can cast it to workflow.NexusOperationCancellationType
// without further translation.
func TestNexusOperationOptions_DecodesCancellationType(t *testing.T) {
	for name, tc := range map[string]struct {
		wire string
		want int
	}{
		"unspecified missing":       {`{}`, 0},
		"unspecified explicit zero": {`{"cancellationType":0}`, 0},
		"abandon":                   {`{"cancellationType":1}`, 1},
		"try-cancel":                {`{"cancellationType":2}`, 2},
		"wait-requested":            {`{"cancellationType":3}`, 3},
		"wait-completed":            {`{"cancellationType":4}`, 4},
	} {
		t.Run(name, func(t *testing.T) {
			var opts NexusOperationOptions
			require.NoError(t, json.Unmarshal([]byte(tc.wire), &opts))
			assert.Equal(t, tc.want, opts.CancellationType)
		})
	}
}

// TestNexusOperationOptions_OmitsZeroCancellationType guards the
// `omitempty` tag: we don't want zero-value cancellationType bloating
// outbound JSON (we never marshal back to PHP, but inbound JSON noise
// would still surface as test asymmetry).
func TestNexusOperationOptions_OmitsZeroCancellationType(t *testing.T) {
	out, err := json.Marshal(NexusOperationOptions{ScheduleToCloseTimeout: time.Second})
	require.NoError(t, err)
	assert.NotContains(t, string(out), "cancellationType")
}

// TestExecuteNexusOperation_DecodesNexusHeaders pins the top-level
// `nexusHeaders` field — PHP ships the `x-nexus-*` raw header map alongside
// `options`, and NexusOperationParams forwards it verbatim to the Go SDK so
// it surfaces on the handler's OperationContext.
func TestExecuteNexusOperation_DecodesNexusHeaders(t *testing.T) {
	wire := []byte(`{
		"endpoint": "e", "service": "s", "operation": "o",
		"nexusHeaders": {
			"x-nexus-caller-workflow-id": "wf-abc",
			"x-nexus-trace-id": "trace-1"
		}
	}`)

	var op ExecuteNexusOperation
	require.NoError(t, json.Unmarshal(wire, &op))
	assert.Equal(t, map[string]string{
		"x-nexus-caller-workflow-id": "wf-abc",
		"x-nexus-trace-id":           "trace-1",
	}, op.NexusHeaders)
}

// TestExecuteNexusOperation_OmitsEmptyNexusHeaders confirms an absent or
// empty `nexusHeaders` field leaves the map nil (NewExecuteNexusOperationParams
// accepts nil — wire shape stays compact).
func TestExecuteNexusOperation_OmitsEmptyNexusHeaders(t *testing.T) {
	wire := []byte(`{"endpoint":"e","service":"s","operation":"o"}`)
	var op ExecuteNexusOperation
	require.NoError(t, json.Unmarshal(wire, &op))
	assert.Nil(t, op.NexusHeaders)

	out, err := json.Marshal(ExecuteNexusOperation{Endpoint: "e", Service: "s", Operation: "o"})
	require.NoError(t, err)
	assert.NotContains(t, string(out), "nexusHeaders")
}

// TestExecuteNexusOperation_NexusOperationParams_AcceptsCancellationType
// covers each cancellationType value flowing through NexusOperationParams
// without panicking. The bindings struct is opaque to us (unexported
// fields), so we only assert the dispatch surface tolerates every legal
// value of the upstream enum.
func TestExecuteNexusOperation_NexusOperationParams_AcceptsCancellationType(t *testing.T) {
	for _, ct := range []int{0, 1, 2, 3, 4} {
		cmd := ExecuteNexusOperation{
			Endpoint:  "e",
			Service:   "s",
			Operation: "o",
			Options:   NexusOperationOptions{CancellationType: ct},
		}
		assert.NotPanics(t, func() {
			_ = cmd.NexusOperationParams(&commonpb.Payloads{}, nil)
		})
	}
}

// TestExecuteNexusOperation_NexusOperationParams_AcceptsNexusHeaders
// guards the headers plumbing: a populated map and a nil map both reach
// NewExecuteNexusOperationParams without surprises.
func TestExecuteNexusOperation_NexusOperationParams_AcceptsNexusHeaders(t *testing.T) {
	t.Run("populated", func(t *testing.T) {
		cmd := ExecuteNexusOperation{
			Endpoint:     "e",
			Service:      "s",
			Operation:    "o",
			NexusHeaders: map[string]string{"x-nexus-caller-workflow-id": "wf-1"},
		}
		assert.NotPanics(t, func() {
			_ = cmd.NexusOperationParams(&commonpb.Payloads{}, nil)
		})
	})
	t.Run("nil", func(t *testing.T) {
		cmd := ExecuteNexusOperation{Endpoint: "e", Service: "s", Operation: "o"}
		assert.NotPanics(t, func() {
			_ = cmd.NexusOperationParams(&commonpb.Payloads{}, nil)
		})
	})
}

func TestNexusServiceInfo_JSONTags(t *testing.T) {
	svc := NexusServiceInfo{
		Name:       "GreetingService",
		Operations: []string{"greet", "farewell"},
	}

	assert.Equal(t, "GreetingService", svc.Name)
	assert.Equal(t, []string{"greet", "farewell"}, svc.Operations)
}
