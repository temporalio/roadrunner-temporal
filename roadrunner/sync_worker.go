package roadrunner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/spiral/goridge/v2"
)

var EmptyPayload = Payload{}

type SyncWorker interface {
	// WorkerBase provides basic functionality for the SyncWorker
	WorkerBase
	// Exec used to execute payload on the SyncWorker
	Exec(ctx context.Context, rqs Payload) (Payload, error)
}

type taskWorker struct {
	mu sync.Mutex
	w  WorkerBase
}

func NewSyncWorker(w WorkerBase) (SyncWorker, error) {
	return &taskWorker{
		w: w,
	}, nil
}

func (tw *taskWorker) Exec(ctx context.Context, rqs Payload) (Payload, error) {
	//tw.mu.Lock()

	if len(rqs.Body) == 0 && len(rqs.Context) == 0 {
		//tw.mu.Unlock()
		return EmptyPayload, fmt.Errorf("payload can not be empty")
	}

	if tw.w.State(ctx).Value() != StateReady {
		//tw.mu.Unlock()
		return EmptyPayload, fmt.Errorf("WorkerProcess is not ready (%s)", tw.w.State(ctx).String())
	}

	tw.w.State(ctx).Set(StateWorking)

	rsp, err := tw.execPayload(ctx, rqs)
	if err != nil {
		if _, ok := err.(TaskError); !ok {
			tw.w.State(ctx).Set(StateErrored)
			tw.w.State(ctx).RegisterExec()
			//tw.mu.Unlock()
			return EmptyPayload, err
		}
		return EmptyPayload, err
	}

	tw.w.State(ctx).Set(StateReady)
	tw.w.State(ctx).RegisterExec()
	//tw.mu.Unlock()
	return rsp, nil
}

func (tw *taskWorker) execPayload(ctx context.Context, rqs Payload) (Payload, error) {
	// two things; todo: merge
	if err := sendControl(tw.w.Relay(ctx), rqs.Context); err != nil {
		return EmptyPayload, errors.Wrap(err, "header error")
	}

	if err := tw.w.Relay(ctx).Send(rqs.Body, 0); err != nil {
		return EmptyPayload, errors.Wrap(err, "sender error")
	}

	var pr goridge.Prefix
	rsp := Payload{}

	var err error
	if rsp.Context, pr, err = tw.w.Relay(ctx).Receive(); err != nil {
		return EmptyPayload, errors.Wrap(err, "WorkerProcess error")
	}

	if !pr.HasFlag(goridge.PayloadControl) {
		return EmptyPayload, fmt.Errorf("malformed WorkerProcess response")
	}

	if pr.HasFlag(goridge.PayloadError) {
		return EmptyPayload, TaskError(rsp.Context)
	}

	// add streaming support :)
	if rsp.Body, pr, err = tw.w.Relay(ctx).Receive(); err != nil {
		return EmptyPayload, errors.Wrap(err, "WorkerProcess error")
	}

	return rsp, nil
}

func (tw *taskWorker) String() string {
	return tw.w.String()
}

func (tw *taskWorker) Created(ctx context.Context) time.Time {
	return tw.w.Created(ctx)
}


func (tw *taskWorker) Pid(ctx context.Context) int64 {
	return tw.w.Pid(ctx)
}

func (tw *taskWorker) State(ctx context.Context) State {
	return tw.w.State(ctx)
}

func (tw *taskWorker) Start(ctx context.Context) error {
	return tw.w.Start(ctx)
}

func (tw *taskWorker) Wait(ctx context.Context) error {
	return tw.w.Wait(ctx)
}


func (tw *taskWorker) Stop(ctx context.Context) error {
	return tw.w.Stop(ctx)
}

func (tw *taskWorker) Kill(ctx context.Context) error {
	return tw.w.Kill(ctx)
}

func (tw *taskWorker) Relay(ctx context.Context) goridge.Relay {
	return tw.w.Relay(ctx)
}

func (tw *taskWorker) AttachRelay(ctx context.Context, rl goridge.Relay) {
	tw.w.AttachRelay(ctx, rl)
}
