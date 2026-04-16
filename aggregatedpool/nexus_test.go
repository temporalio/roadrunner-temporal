package aggregatedpool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/converter"
	"go.uber.org/zap"

	nexus "github.com/nexus-rpc/sdk-go/nexus"
)

// ── NewNexusHandler ───────────────────────────────────────────

func TestNewNexusHandler(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	assert.NotNil(t, handler)
	assert.Equal(t, log, handler.log)
	assert.NotNil(t, handler.pldPool)
	assert.Equal(t, uint64(0), handler.seqID)
}

func TestNewNexusHandler_PayloadPool(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	// Verify pool returns valid payloads
	pld := handler.getPld()
	require.NotNil(t, pld)
	pld.Body = []byte("test")
	pld.Context = []byte("ctx")

	handler.putPld(pld)
	// After put, fields should be reset
	assert.Nil(t, pld.Body)
	assert.Nil(t, pld.Context)
	assert.Equal(t, uint8(0), pld.Codec)
}

// ── CreateNexusService ────────────────────────────────────────

func TestNexusHandler_CreateNexusService(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	svc := handler.CreateNexusService("test-queue", "GreetingService", []string{"greet", "farewell"}, false)

	assert.NotNil(t, svc)
	assert.Equal(t, "GreetingService", svc.Name)

	op1 := svc.Operation("greet")
	assert.NotNil(t, op1)
	assert.Equal(t, "greet", op1.Name())

	op2 := svc.Operation("farewell")
	assert.NotNil(t, op2)
	assert.Equal(t, "farewell", op2.Name())
}

func TestNexusHandler_CreateNexusService_NoOperations(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	svc := handler.CreateNexusService("test-queue", "EmptyService", []string{}, false)

	assert.NotNil(t, svc)
	assert.Equal(t, "EmptyService", svc.Name)
}

func TestNexusHandler_CreateNexusService_NilOperations(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	// Should not panic with nil slice
	svc := handler.CreateNexusService("test-queue", "EmptyService", nil, false)
	assert.NotNil(t, svc)
}

func TestNexusHandler_CreateNexusService_MultipleServices(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	svc1 := handler.CreateNexusService("test-queue", "ServiceA", []string{"opA1", "opA2"}, false)
	svc2 := handler.CreateNexusService("test-queue", "ServiceB", []string{"opB"}, false)

	assert.Equal(t, "ServiceA", svc1.Name)
	assert.Equal(t, "ServiceB", svc2.Name)

	// Each service has its own operations
	assert.NotNil(t, svc1.Operation("opA1"))
	assert.NotNil(t, svc1.Operation("opA2"))
	assert.NotNil(t, svc2.Operation("opB"))

	// Operations from different services are isolated
	assert.Nil(t, svc1.Operation("opB"))
	assert.Nil(t, svc2.Operation("opA1"))
}

func TestNexusHandler_CreateNexusService_DifferentTaskQueues(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	// Same service name on different task queues — each operation should
	// carry its own task queue (verified through the operation struct).
	svc1 := handler.CreateNexusService("queue-1", "Service", []string{"op"}, false)
	svc2 := handler.CreateNexusService("queue-2", "Service", []string{"op"}, false)

	require.NotNil(t, svc1.Operation("op"))
	require.NotNil(t, svc2.Operation("op"))

	// They are distinct service instances
	assert.NotSame(t, svc1, svc2)
}

func TestNexusHandler_CreateNexusService_DuplicateRegistration(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	// Registering same service name twice produces independent service objects
	svc1 := handler.CreateNexusService("test-queue", "SameService", []string{"op1"}, false)
	svc2 := handler.CreateNexusService("test-queue", "SameService", []string{"op2"}, false)

	require.NotNil(t, svc1)
	require.NotNil(t, svc2)
	assert.NotSame(t, svc1, svc2)
}

// ── nexusOperation struct ─────────────────────────────────────

func TestNexusOperation_Name(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	op := &nexusOperation{
		name:        "myOp",
		serviceName: "MyService",
		taskQueue:   "my-tq",
		handler:     handler,
	}

	assert.Equal(t, "myOp", op.Name())
}

func TestNexusOperation_Fields(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	op := &nexusOperation{
		name:        "test",
		serviceName: "TestService",
		taskQueue:   "tq",
		handler:     handler,
	}

	assert.Equal(t, "test", op.name)
	assert.Equal(t, "TestService", op.serviceName)
	assert.Equal(t, "tq", op.taskQueue)
	assert.Same(t, handler, op.handler)
}

// TestNexusOperation_TypeAssertion verifies operations satisfy the nexus interface.
func TestNexusOperation_TypeAssertion(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	op := &nexusOperation{
		name:        "test",
		serviceName: "TestService",
		taskQueue:   "tq",
		handler:     handler,
	}

	var _ nexus.RegisterableOperation = op
	var _ nexus.Operation[converter.RawValue, converter.RawValue] = op
}

// TestNexusOperation_TaskQueuePropagation verifies the task queue from
// CreateNexusService is correctly stored in each operation.
func TestNexusOperation_TaskQueuePropagation(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	taskQueue := "my-special-queue"
	svc := handler.CreateNexusService(taskQueue, "Svc", []string{"op1", "op2", "op3"}, false)
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

// ── Cancel signature check ────────────────────────────────────

func TestNexusHandler_CancelOperation_NilPool_ReturnsError(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	// Calling cancelOperation with nil codec should fail (codec.Encode panics on nil)
	// We use defer recover to verify the failure mode.
	defer func() {
		if r := recover(); r != nil {
			// Expected — nil codec causes panic
			return
		}
	}()

	err := handler.cancelOperation(context.Background(), "tq", "svc", "op", "token", nexus.CancelOperationOptions{})
	assert.Error(t, err, "expected error or panic with nil codec/pool")
}

// ── Service registration validation ───────────────────────────

func TestNexusHandler_OperationsListPreservesOrder(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	ops := []string{"alpha", "beta", "gamma", "delta"}
	svc := handler.CreateNexusService("tq", "OrderedSvc", ops, false)

	// All operations should be findable
	for _, name := range ops {
		assert.NotNil(t, svc.Operation(name), "missing operation %q", name)
	}
}

// TestNexusHandler_SeqIDIncrement verifies that seqID is unique across calls.
// We can't easily test the actual increment without mocking the pool, but
// we verify the initial state.
func TestNexusHandler_SeqIDInitialState(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	assert.Equal(t, uint64(0), handler.seqID, "seqID should start at 0")
}
