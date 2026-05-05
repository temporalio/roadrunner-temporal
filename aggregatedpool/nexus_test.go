package aggregatedpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/converter"
	"go.uber.org/zap"

	nexus "github.com/nexus-rpc/sdk-go/nexus"
)

// putPld must clear the payload before returning it to the pool — otherwise
// stale Body/Context bytes leak into the next Encode call.
func TestNexusHandler_PayloadPoolResetsOnPut(t *testing.T) {
	handler := NewNexusHandler(nil, nil, zap.NewNop())

	pld := handler.getPld()
	pld.Body = []byte("test")
	pld.Context = []byte("ctx")
	pld.Codec = 7

	handler.putPld(pld)
	assert.Nil(t, pld.Body)
	assert.Nil(t, pld.Context)
	assert.Equal(t, uint8(0), pld.Codec)
}

func TestNexusHandler_CreateNexusService_RegistersOperations(t *testing.T) {
	handler := NewNexusHandler(nil, nil, zap.NewNop())
	svc := handler.CreateNexusService("tq", "GreetingService", []string{"greet", "farewell"})

	require.NotNil(t, svc.Operation("greet"))
	require.NotNil(t, svc.Operation("farewell"))
	assert.Nil(t, svc.Operation("missing"))
}

func TestNexusHandler_CreateNexusService_AcceptsEmptyAndNilOperations(t *testing.T) {
	handler := NewNexusHandler(nil, nil, zap.NewNop())
	require.NotPanics(t, func() { handler.CreateNexusService("tq", "S", nil) })
	require.NotPanics(t, func() { handler.CreateNexusService("tq", "S", []string{}) })
}

func TestNexusHandler_CreateNexusService_IsolatesOperationsBetweenServices(t *testing.T) {
	handler := NewNexusHandler(nil, nil, zap.NewNop())
	a := handler.CreateNexusService("tq", "ServiceA", []string{"opA"})
	b := handler.CreateNexusService("tq", "ServiceB", []string{"opB"})

	assert.NotNil(t, a.Operation("opA"))
	assert.Nil(t, a.Operation("opB"))
	assert.NotNil(t, b.Operation("opB"))
	assert.Nil(t, b.Operation("opA"))
}

// Compile-time guarantee that nexusOperation satisfies the nexus SDK interfaces.
var (
	_ nexus.RegisterableOperation                             = (*nexusOperation)(nil)
	_ nexus.Operation[converter.RawValue, converter.RawValue] = (*nexusOperation)(nil)
)

// TaskQueue from CreateNexusService must reach each operation — that's the
// link the dispatch path follows when a task arrives.
func TestNexusOperation_TaskQueuePropagation(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	taskQueue := "my-special-queue"
	svc := handler.CreateNexusService(taskQueue, "Svc", []string{"op1", "op2", "op3"})
	require.NotNil(t, svc)

	for _, opName := range []string{"op1", "op2", "op3"} {
		op := svc.Operation(opName)
		require.NotNil(t, op)

		// Type-assert back to our concrete type to verify task queue
		concrete, ok := op.(*nexusOperation)
		require.True(t, ok, "operation %q is not *nexusOperation", opName)
		assert.Equal(t, taskQueue, concrete.taskQueue, "operation %q has wrong task queue", opName)
		assert.Equal(t, "Svc", concrete.serviceName, "operation %q has wrong service name", opName)
		assert.Equal(t, opName, concrete.name)
	}
}
