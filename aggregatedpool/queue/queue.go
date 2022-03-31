package queue

import (
	"github.com/temporalio/roadrunner-temporal/internal"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/api/failure/v1"
)

type MessageQueue struct {
	SeqID func() uint64
	Queue []*internal.Message
}

func NewMessageQueue(sedID func() uint64) *MessageQueue {
	return &MessageQueue{
		SeqID: sedID,
		Queue: make([]*internal.Message, 0, 5),
	}
}

func (mq *MessageQueue) Flush() {
	mq.Queue = mq.Queue[0:0]
}

// TODO(rustatian) allocate??? -> to sync.Pool
func (mq *MessageQueue) AllocateMessage(cmd interface{}, payloads *common.Payloads, header *common.Header) (uint64, internal.Message) {
	msg := internal.Message{
		ID:       mq.SeqID(),
		Command:  cmd,
		Payloads: payloads,
		Header:   header,
	}

	return msg.ID, msg
}

func (mq *MessageQueue) PushCommand(cmd interface{}, payloads *common.Payloads, header *common.Header) uint64 {
	id := mq.SeqID()
	mq.Queue = append(mq.Queue, &internal.Message{
		ID:       id,
		Command:  cmd,
		Failure:  nil,
		Payloads: payloads,
		Header:   header,
	})
	return id
}

func (mq *MessageQueue) PushResponse(id uint64, payloads *common.Payloads) {
	mq.Queue = append(mq.Queue, &internal.Message{ID: id, Payloads: payloads})
}

func (mq *MessageQueue) PushError(id uint64, failure *failure.Failure) {
	mq.Queue = append(mq.Queue, &internal.Message{ID: id, Failure: failure})
}
