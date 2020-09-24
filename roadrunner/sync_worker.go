package roadrunner

import (
	"errors"
	"fmt"
	"github.com/spiral/goridge/v2"
	"sync"
)

type SyncWorker interface {
	//Worker todo: implement

	// blocking: todo: remove pointers
	Exec(rqs *Payload) (rsp *Payload, err error)
}

type taskWorker struct {
	mu sync.Mutex
	w  Worker
}

func NewSyncWorker(w Worker) (SyncWorker, error) {
	return &taskWorker{w: w}, nil
}

func (tw *taskWorker) Exec(rqs *Payload) (rsp *Payload, err error) {
	tw.mu.Lock()

	if rqs == nil {
		tw.mu.Unlock()
		return nil, fmt.Errorf("payload can not be empty")
	}

	if tw.w.State().Value() != StateReady {
		tw.mu.Unlock()
		return nil, fmt.Errorf("WorkerProcess is not ready (%s)", tw.w.State().String())
	}

	tw.w.State().Set(StateWorking)

	rsp, err = tw.execPayload(rqs)
	if err != nil {
		if _, ok := err.(TaskError); !ok {
			tw.w.State().Set(StateErrored)
			tw.w.State().RegisterExec()
			tw.mu.Unlock()
			return nil, err
		}
	}

	tw.w.State().Set(StateReady)
	tw.w.State().RegisterExec()
	tw.mu.Unlock()
	return rsp, err
}

func (tw *taskWorker) execPayload(rqs *Payload) (rsp *Payload, err error) {
	// two things; todo: merge
	if err := sendControl(tw.w.Relay(), rqs.Context); err != nil {
		return nil, errors.Wrap(err, "header error")
	}

	if err = tw.w.Relay().Send(rqs.Body, 0); err != nil {
		return nil, errors.Wrap(err, "sender error")
	}

	var pr goridge.Prefix
	rsp = new(Payload)

	if rsp.Context, pr, err = tw.w.Relay().Receive(); err != nil {
		return nil, errors.Wrap(err, "WorkerProcess error")
	}

	if !pr.HasFlag(goridge.PayloadControl) {
		return nil, fmt.Errorf("malformed WorkerProcess response")
	}

	if pr.HasFlag(goridge.PayloadError) {
		return nil, TaskError(rsp.Context)
	}

	// add streaming support :)
	if rsp.Body, pr, err = tw.w.Relay().Receive(); err != nil {
		return nil, errors.Wrap(err, "WorkerProcess error")
	}

	return rsp, nil
}
