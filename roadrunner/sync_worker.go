package roadrunner

import (
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/spiral/goridge/v2"
)

var EmptyPayload = Payload{}

type SyncWorker interface {
	// Worker provides basic functionality for the SyncWorker
	Worker
	// Exec used to execute payload on the SyncWorker
	Exec(rqs Payload) (rsp Payload, err error)
}

type taskWorker struct {
	mu sync.Mutex
	w  Worker
}

func NewSyncWorker(w Worker) (SyncWorker, error) {
	return &taskWorker{w: w}, nil
}

func (tw *taskWorker) Exec(rqs Payload) (rsp Payload, err error) {
	tw.mu.Lock()

	if len(rqs.Body) == 0 && len(rqs.Context) == 0 {
		tw.mu.Unlock()
		return EmptyPayload, fmt.Errorf("payload can not be empty")
	}

	if tw.w.State().Value() != StateReady {
		tw.mu.Unlock()
		return EmptyPayload, fmt.Errorf("WorkerProcess is not ready (%s)", tw.w.State().String())
	}

	tw.w.State().Set(StateWorking)

	rsp, err = tw.execPayload(rqs)
	if err != nil {
		if _, ok := err.(TaskError); !ok {
			tw.w.State().Set(StateErrored)
			tw.w.State().RegisterExec()
			tw.mu.Unlock()
			return EmptyPayload, err
		}
	}

	tw.w.State().Set(StateReady)
	tw.w.State().RegisterExec()
	tw.mu.Unlock()
	return rsp, err
}

func (tw *taskWorker) execPayload(rqs Payload) (Payload, error) {
	// two things; todo: merge
	if err := sendControl(tw.w.Relay(), rqs.Context); err != nil {
		return EmptyPayload, errors.Wrap(err, "header error")
	}

	if err := tw.w.Relay().Send(rqs.Body, 0); err != nil {
		return EmptyPayload, errors.Wrap(err, "sender error")
	}

	var pr goridge.Prefix
	rsp := Payload{}

	var err error
	if rsp.Context, pr, err = tw.w.Relay().Receive(); err != nil {
		return EmptyPayload, errors.Wrap(err, "WorkerProcess error")
	}

	if !pr.HasFlag(goridge.PayloadControl) {
		return EmptyPayload, fmt.Errorf("malformed WorkerProcess response")
	}

	if pr.HasFlag(goridge.PayloadError) {
		return EmptyPayload, TaskError(rsp.Context)
	}

	// add streaming support :)
	if rsp.Body, pr, err = tw.w.Relay().Receive(); err != nil {
		return EmptyPayload, errors.Wrap(err, "WorkerProcess error")
	}

	return rsp, nil
}

func (tw *taskWorker) String() string {
	return tw.w.String()
}

func (tw *taskWorker) Created() time.Time {
	return tw.w.Created()
}

func (tw *taskWorker) Events() <-chan WorkerEvent {
	return tw.w.Events()
}

func (tw *taskWorker) Pid() int64 {
	return tw.w.Pid()
}

func (tw *taskWorker) State() State {
	return tw.w.State()
}

func (tw *taskWorker) Start() error {
	return tw.w.Start()
}

func (tw *taskWorker) Wait() error {
	return tw.w.Wait()
}

func (tw *taskWorker) WaitChan() chan interface{} {
	return tw.w.WaitChan()
}

func (tw *taskWorker) Stop() error {
	return tw.w.Stop()
}

func (tw *taskWorker) Kill() error {
	return tw.w.Kill()
}

func (tw *taskWorker) Relay() goridge.Relay {
	return tw.w.Relay()
}

func (tw *taskWorker) AttachRelay(rl goridge.Relay) {
	tw.w.AttachRelay(rl)
}
