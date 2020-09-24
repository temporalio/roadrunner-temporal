package roadrunner

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/spiral/goridge/v2"
	"io"
	"os/exec"
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
func (f *PipeFactory) SpawnWorker(cmd *exec.Cmd) (w Worker, err error) {
	if w, err = initWorker(cmd); err != nil {
		return nil, err
	}

	var (
		in  io.ReadCloser
		out io.WriteCloser
	)

	if in, err = cmd.StdoutPipe(); err != nil {
		return nil, err
	}

	if out, err = cmd.StdinPipe(); err != nil {
		return nil, err
	}

	relay := goridge.NewPipeRelay(in, out)
	w.AttachRelay(relay)

	if err := w.Start(); err != nil {
		return nil, errors.Wrap(err, "process error")
	}

	if pid, err := fetchPID(relay); pid != w.Pid() {
		go func(w Worker) {
			err := w.Kill()
			if err != nil {
				// there is no logger here, how to handle error in goroutines ?
				fmt.Println(
					fmt.Sprintf(
						"error killing the WorkerProcess with PID number %d, created: %s",
						w.Pid(),
						w.Created(),
					),
				)
			}
		}(w)

		if wErr := w.Wait(); wErr != nil {
			if _, ok := wErr.(*exec.ExitError); ok {
				// error might be nil here
				if err != nil {
					err = errors.Wrap(wErr, err.Error())
				}
			} else {
				err = wErr
			}
		}

		return nil, errors.Wrap(err, "unable to connect to WorkerProcess")
	}

	w.State().Set(StateReady)
	return w, nil
}

// Close the factory.
func (f *PipeFactory) Close() error {
	return nil
}
