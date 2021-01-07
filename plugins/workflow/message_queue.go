package workflow

import (
	rrt "github.com/temporalio/roadrunner-temporal/protocol"
	"go.temporal.io/api/common/v1"
)

type messageQueue struct {
	seqID func() uint64
	queue []rrt.Message
}

func newMessageQueue(sedID func() uint64) *messageQueue {
	return &messageQueue{
		seqID: sedID,
		queue: make([]rrt.Message, 0, 5),
	}
}

func (mq *messageQueue) flush() {
	mq.queue = mq.queue[0:0]
}

func (mq *messageQueue) allocateMessage(cmd interface{}) (id uint64, msg rrt.Message, err error) {
	msg = rrt.Message{ID: mq.seqID(), Command: cmd}

	return msg.ID, msg, nil
}

func (mq *messageQueue) pushCommand(cmd interface{}) (id uint64, err error) {
	id, msg, err := mq.allocateMessage(cmd)
	if err != nil {
		return 0, err
	}

	mq.queue = append(mq.queue, msg)

	return id, nil
}

func (mq *messageQueue) pushResponse(id uint64, result []*common.Payload) {
	mq.queue = append(mq.queue, rrt.Message{ID: id, Result: result})
}

func (mq *messageQueue) pushPayloadsResponse(id uint64, result *common.Payloads) {
	if result == nil {
		mq.queue = append(mq.queue, rrt.Message{ID: id, Result: []*common.Payload{}})
	} else {
		mq.queue = append(mq.queue, rrt.Message{ID: id, Result: result.Payloads})
	}
}

func (mq *messageQueue) pushError(id uint64, err error) {
	mq.queue = append(mq.queue, rrt.Message{
		ID: id,
		Error: &rrt.Error{
			Message: err.Error(),
		},
	})
}
