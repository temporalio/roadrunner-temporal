package queue

import (
	"sync"

	"github.com/temporalio/roadrunner-temporal/v2/internal"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/api/failure/v1"
)

type MessageQueue struct {
	SeqID func() uint64
	mu    sync.Mutex
	queue []*internal.Message
}

func NewMessageQueue(sedID func() uint64) *MessageQueue {
	return &MessageQueue{
		SeqID: sedID,
		queue: make([]*internal.Message, 0, 5),
	}
}

func (mq *MessageQueue) Flush() {
	mq.mu.Lock()
	mq.queue = mq.queue[:0]
	mq.mu.Unlock()
}

// AllocateMessage ..
// TODO(rustatian) allocate??? -> to sync.Pool
// Remove this method if flavor of sync.Pool with internal.Message
func (mq *MessageQueue) AllocateMessage(cmd any, payloads *common.Payloads, header *common.Header) *internal.Message {
	msg := &internal.Message{
		ID:       mq.SeqID(),
		Command:  cmd,
		Payloads: payloads,
		Header:   header,
	}

	return msg
}

func (mq *MessageQueue) PushCommand(cmd any, payloads *common.Payloads, header *common.Header) {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	mq.queue = append(mq.queue, &internal.Message{
		ID:       mq.SeqID(),
		Command:  cmd,
		Payloads: payloads,
		Header:   header,
	})
}

func (mq *MessageQueue) PushResponse(id uint64, payloads *common.Payloads) {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	mq.queue = append(mq.queue, &internal.Message{
		ID:       id,
		Payloads: payloads,
	})
}

func (mq *MessageQueue) PushError(id uint64, failure *failure.Failure) {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	mq.queue = append(mq.queue, &internal.Message{ID: id, Failure: failure})
}

func (mq *MessageQueue) Messages() []*internal.Message {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	return mq.queue
}
