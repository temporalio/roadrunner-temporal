package workflow

import (
	"encoding/json"
	rrt "github.com/temporalio/roadrunner-temporal"
	"sync/atomic"
)

type messageQueue struct {
	seqID *uint64
	queue []rrt.Message
}

func newMessageQueue(sedID *uint64) *messageQueue {
	return &messageQueue{
		seqID: sedID,
		queue: make([]rrt.Message, 0, 5),
	}
}

func (mq *messageQueue) flush() {
	mq.queue = mq.queue[0:0]
}

func (mq *messageQueue) pushCommand(cmd string, params interface{}) (id uint64, err error) {
	msg := rrt.Message{ID: atomic.AddUint64(mq.seqID, 1), Command: cmd}

	msg.Params, err = json.Marshal(params)
	if err != nil {
		return 0, err
	}

	mq.queue = append(mq.queue, msg)

	return id, nil
}

func (mq *messageQueue) makeCommand(cmd string, params interface{}) (id uint64, msg rrt.Message, err error) {
	msg = rrt.Message{ID: atomic.AddUint64(mq.seqID, 1), Command: cmd}

	msg.Params, err = json.Marshal(params)
	if err != nil {
		return 0, rrt.Message{}, err
	}

	return id, msg, nil
}

func (mq *messageQueue) pushResponse(id uint64, result []json.RawMessage) {
	mq.queue = append(mq.queue, rrt.Message{ID: id, Result: result})
}

func (mq *messageQueue) pushError(id uint64, err error) {
	// todo: implement on later stage

	//	cmd := rrt.Message{
	//		ID:    id,
	//		Payload: err.Payload(),
	//}
	//log.Println("error!", err)
	//wp.mq = append(wp.mq, cmd)

	// todo: implement
}
