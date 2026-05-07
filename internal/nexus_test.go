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
		{"GetNexusOperationStarted value", GetNexusOperationStarted{}, "GetNexusOperationStarted"},
		{"GetNexusOperationStarted ptr", &GetNexusOperationStarted{}, "GetNexusOperationStarted"},
		{"NexusOperationStarted value", NexusOperationStarted{}, "NexusOperationStarted"},
		{"NexusOperationStarted ptr", &NexusOperationStarted{}, "NexusOperationStarted"},
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
		{"GetNexusOperationStarted", "GetNexusOperationStarted", &GetNexusOperationStarted{}},
		{"NexusOperationStarted", "NexusOperationStarted", &NexusOperationStarted{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InitCommand(tt.typeName)
			require.NoError(t, err)
			assert.IsType(t, tt.wantType, got)
		})
	}
}

// TestGetNexusOperationStarted_DecodesPHPWireShape pins the JSON contract with
// the PHP Internal\Transport\Request\GetNexusOperationStarted: a single `id`
// field carrying the original ExecuteNexusOperation message ID.
func TestGetNexusOperationStarted_DecodesPHPWireShape(t *testing.T) {
	wire := []byte(`{"id":42}`)
	var cmd GetNexusOperationStarted
	require.NoError(t, json.Unmarshal(wire, &cmd))
	assert.Equal(t, uint64(42), cmd.ID)
}

// TestGetNexusOperationStarted_RoundTrip ensures we can also marshal back to
// the same wire shape — useful if RR ever needs to log or echo the request.
func TestGetNexusOperationStarted_RoundTrip(t *testing.T) {
	cmd := GetNexusOperationStarted{ID: 77}
	data, err := json.Marshal(cmd)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":77}`, string(data))
}

// TestInitCommand_RemovedPollingCommands guards against a regression that
// would silently re-introduce the old polling protocol. RR no longer
// understands these names — they must come back as "undefined command".
func TestInitCommand_RemovedPollingCommands(t *testing.T) {
	for _, removed := range []string{"GetNexusOperationResult", "CancelNexusOperationResult"} {
		t.Run(removed, func(t *testing.T) {
			got, err := InitCommand(removed)
			assert.Error(t, err, "removed command %q must not decode anymore", removed)
			assert.Nil(t, got)
		})
	}
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

// TestNexusOperationStarted_SyncWireShape pins the PHP→Go reply JSON shape
// for the sync-success case. Async=false, Token must be omitted, Links may
// be empty/absent.
func TestNexusOperationStarted_SyncWireShape(t *testing.T) {
	out, err := json.Marshal(NexusOperationStarted{Async: false})
	require.NoError(t, err)
	assert.JSONEq(t, `{"async":false}`, string(out))

	out, err = json.Marshal(NexusOperationStarted{
		Async: false,
		Links: []NexusLink{{URL: "http://x/y", Type: "t"}},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"async":false,"links":[{"url":"http://x/y","type":"t"}]}`, string(out))
}

// TestNexusOperationStarted_AsyncWireShape pins the PHP→Go reply JSON shape
// for the async-success case. Async=true, Token populated.
func TestNexusOperationStarted_AsyncWireShape(t *testing.T) {
	out, err := json.Marshal(NexusOperationStarted{
		Async: true,
		Token: "tok-abc",
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"async":true,"token":"tok-abc"}`, string(out))
}

// TestNexusOperationStarted_DecodesPHPWireShape pins decode of the typed
// reply emitted by the PHP route in place of the legacy _rr_nexus_*
// payload-metadata markers.
func TestNexusOperationStarted_DecodesPHPWireShape(t *testing.T) {
	wire := []byte(`{"async":true,"token":"op-1","links":[{"url":"http://a/b","type":"x.y"}]}`)
	var reply NexusOperationStarted
	require.NoError(t, json.Unmarshal(wire, &reply))
	assert.True(t, reply.Async)
	assert.Equal(t, "op-1", reply.Token)
	require.Len(t, reply.Links, 1)
	assert.Equal(t, "http://a/b", reply.Links[0].URL)
	assert.Equal(t, "x.y", reply.Links[0].Type)
}

// InvocationID is the cooperative-cancel correlation key and must always
// be present on the wire — PHP needs it for CancelNexusOperationMethod.
func TestInvokeNexusOperation_InvocationIDAlwaysPresent(t *testing.T) {
	zero, err := json.Marshal(InvokeNexusOperation{Service: "S", Operation: "o"})
	require.NoError(t, err)
	assert.Contains(t, string(zero), `"invocationId":0`)

	set, err := json.Marshal(InvokeNexusOperation{Service: "S", Operation: "o", InvocationID: 99})
	require.NoError(t, err)
	assert.Contains(t, string(set), `"invocationId":99`)
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
func TestWorkerInfo_NexusServicesJSON(t *testing.T) {
	jsonPayload := []byte(`{
		"TaskQueue": "test-queue",
		"nexusServices": [
			{"name": "GreetingService", "operations": ["greet", "farewell"]},
			{"name": "EchoService", "operations": ["echo"]}
		]
	}`)

	var wi WorkerInfo
	require.NoError(t, json.Unmarshal(jsonPayload, &wi))

	assert.Equal(t, "test-queue", wi.TaskQueue)
	assert.Len(t, wi.NexusServices, 2)

	assert.Equal(t, "GreetingService", wi.NexusServices[0].Name)
	assert.Equal(t, []string{"greet", "farewell"}, wi.NexusServices[0].Operations)

	assert.Equal(t, "EchoService", wi.NexusServices[1].Name)
	assert.Equal(t, []string{"echo"}, wi.NexusServices[1].Operations)
}

func TestWorkerInfo_EmptyNexusServices(t *testing.T) {
	jsonPayload := []byte(`{"TaskQueue": "test-queue"}`)

	var wi WorkerInfo
	require.NoError(t, json.Unmarshal(jsonPayload, &wi))

	assert.Equal(t, "test-queue", wi.TaskQueue)
	assert.Empty(t, wi.NexusServices)
}

func TestNexusServiceInfo_JSONMarshal(t *testing.T) {
	svc := NexusServiceInfo{
		Name:       "GreetingService",
		Operations: []string{"greet"},
	}

	data, err := json.Marshal(svc)
	require.NoError(t, err)

	assert.JSONEq(t, `{"name":"GreetingService","operations":["greet"]}`, string(data))
}

// TestWorkerInfo_NexusServicesJSONLowercase guards Go's case-insensitive
// JSON unmarshal: PHP may ship `nexusServices` (camelCase) and Go must still
// populate the PascalCase-tagged field.
func TestWorkerInfo_NexusServicesJSONLowercase(t *testing.T) {
	jsonPayload := []byte(`{"nexusServices":[{"name":"S","operations":["o"]}]}`)
	var wi WorkerInfo
	require.NoError(t, json.Unmarshal(jsonPayload, &wi))
	require.Len(t, wi.NexusServices, 1)
	assert.Equal(t, "S", wi.NexusServices[0].Name)
}
