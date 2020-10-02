package roadrunner

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/spiral/goridge/v2"
)

// PipeFactory connects to workers using standard
// streams (STDIN, STDOUT pipes).
type PipeFactory struct {
}

// NewPipeFactory returns new factory instance and starts
// listening

// todo: review tests
func NewPipeFactory() *PipeFactory {
	return &PipeFactory{}
}

// SpawnWorker creates new WorkerProcess and connects it to goridge relay,
// method Wait() must be handled on level above.
func (f *PipeFactory) SpawnWorker(ctx context.Context, cmd *exec.Cmd) (WorkerBase, error) {
	w, err := initWorker(ctx, cmd)
	if err != nil {
		return nil, err
	}

	// TODO why out is in?
	in, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	// TODO why in is out?
	out, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	// Init new PIPE relay
	relay := goridge.NewPipeRelay(in, out)
	w.AttachRelay(ctx, relay)

	// Start the worker
	err = w.Start(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "process error")
	}

	pid, err := fetchPID(relay)
	if err != nil {
		return nil, err
	}

	if pid != w.Pid(ctx) {
		err = w.Kill(ctx)
		if err != nil {
			return nil, errors.New(fmt.Sprintf(
				"error killing the WorkerProcess with PID number %d, created: %s",
				w.Pid(ctx),
				w.Created(ctx),
			))
		}
	}

	w.State(ctx).Set(StateReady)

	return w, nil
}

func (f *PipeFactory) Listen() {

}

// Close the factory.
func (f *PipeFactory) Close(ctx context.Context) error {
	return nil
}
