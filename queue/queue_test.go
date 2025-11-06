package queue

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/temporalio/roadrunner-temporal/v5/internal"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/api/failure/v1"
)

func Test_MessageQueueFlushError(t *testing.T) {
	var index uint64
	mq := NewMessageQueue(func() uint64 {
		return atomic.AddUint64(&index, 1)
	})

	mq.PushError(1, &failure.Failure{})
	assert.Len(t, mq.Messages(), 1)

	mq.Flush()
	assert.Len(t, mq.Messages(), 0)
	assert.Equal(t, uint64(0), index)
}

func Test_MessageQueueFlushResponse(t *testing.T) {
	var index uint64
	mq := NewMessageQueue(func() uint64 {
		return atomic.AddUint64(&index, 1)
	})

	mq.PushResponse(1, &common.Payloads{})
	assert.Len(t, mq.Messages(), 1)

	mq.Flush()
	assert.Len(t, mq.Messages(), 0)
	assert.Equal(t, uint64(0), index)
}

func Test_MessageQueueCommandID(t *testing.T) {
	var index uint64
	mq := NewMessageQueue(func() uint64 {
		return atomic.AddUint64(&index, 1)
	})

	mq.PushCommand(internal.StartWorkflow{}, &common.Payloads{}, &common.Header{}, 0)
	assert.Len(t, mq.Messages(), 1)

	mq.Flush()
	assert.Len(t, mq.Messages(), 0)
}
