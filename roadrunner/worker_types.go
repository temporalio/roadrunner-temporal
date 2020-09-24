package roadrunner

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/spiral/goridge/v2"
)

type TaskWorker interface {
	// blocking
	Exec(rqs *Payload) (rsp *Payload, err error)
}

type taskWorker struct {
	w *Worker
}

func (tw *taskWorker) Exec(rqs *Payload) (rsp *Payload, err error) {
	tw.w.mu.Lock()

	if rqs == nil {
		tw.w.mu.Unlock()
		return nil, fmt.Errorf("payload can not be empty")
	}

	if tw.w.state.Value() != StateReady {
		tw.w.mu.Unlock()
		return nil, fmt.Errorf("worker is not ready (%s)", tw.w.state.String())
	}

	tw.w.state.set(StateWorking)

	rsp, err = tw.execPayload(rqs)
	if err != nil {
		if _, ok := err.(TaskError); !ok {
			tw.w.state.set(StateErrored)
			tw.w.state.registerExec()
			tw.w.mu.Unlock()
			return nil, err
		}
	}

	tw.w.state.set(StateReady)
	tw.w.state.registerExec()
	tw.w.mu.Unlock()
	return rsp, err
}

func (tw *taskWorker) execPayload(rqs *Payload) (rsp *Payload, err error) {
	// two things
	if err := sendControl(tw.w.rl, rqs.Context); err != nil {
		return nil, errors.Wrap(err, "header error")
	}

	if err = tw.w.rl.Send(rqs.Body, 0); err != nil {
		return nil, errors.Wrap(err, "sender error")
	}

	var pr goridge.Prefix
	rsp = new(Payload)

	if rsp.Context, pr, err = tw.w.rl.Receive(); err != nil {
		return nil, errors.Wrap(err, "worker error")
	}

	if !pr.HasFlag(goridge.PayloadControl) {
		return nil, fmt.Errorf("malformed worker response")
	}

	if pr.HasFlag(goridge.PayloadError) {
		return nil, TaskError(rsp.Context)
	}

	// add streaming support :)
	if rsp.Body, pr, err = tw.w.rl.Receive(); err != nil {
		return nil, errors.Wrap(err, "worker error")
	}

	return rsp, nil
}

func NewTaskWorker(w *Worker) TaskWorker {
	return &taskWorker{w}
}

// todo: implement
type AsyncWorker interface {
	OnReceive(func())
}
