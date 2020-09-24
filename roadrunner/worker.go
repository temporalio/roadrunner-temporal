package roadrunner

import (
	"fmt"
	"github.com/spiral/goridge/v2"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// EventWorkerKill thrown after worker is being forcefully killed.
const (
	// EventWorkerError triggered after worker. Except payload to be error.
	EventWorkerError = iota + 100

	// EventWorkerLog triggered on every write to worker StdErr pipe (batched). Except payload to be []byte string.
	EventWorkerLog

	// WaitDuration - for how long error buffer should attempt to aggregate error messages
	// before merging output together since lastError update (required to keep error update together).
	WaitDuration = 100 * time.Millisecond
)

type WorkerEvent struct {
	Event   int
	Worker  *Worker
	Payload interface{}
}

// Worker - supervised process with api over goridge.Relay.
type Worker struct {
	// Pid of the process, points to Pid of underlying process and
	// can be nil while process is not started.
	Pid *int

	// Created indicates at what time worker has been created.
	Created time.Time

	// updates parent supervisor or pool about worker events
	Events chan<- WorkerEvent

	// state holds information about current worker state,
	// number of worker executions, buf status change time.
	// publicly this object is receive-only and protected using Mutex
	// and atomic counter.
	state *state

	// underlying command with associated process, command must be
	// provided to worker from outside in non-started form. CmdSource
	// stdErr direction will be handled by worker to aggregate error message.
	cmd *exec.Cmd

	// errBuffer aggregates stderr output from underlying process. Value can be
	// receive only once command is completed and all pipes are closed.
	errBuffer *errBuffer

	// channel is being closed once command is complete.
	waitDone chan interface{}

	// contains information about resulted process state.
	endState *os.ProcessState

	// ensures than only one execution can be run at once.
	mu sync.Mutex

	// communication bus with underlying process.
	rl goridge.Relay
}

// initWorker creates new worker over given exec.cmd.
func initWorker(cmd *exec.Cmd) (*Worker, error) {
	if cmd.Process != nil {
		return nil, fmt.Errorf("can't attach to running process")
	}

	w := &Worker{
		Created:  time.Now(),
		cmd:      cmd,
		Events:   make(chan WorkerEvent),
		waitDone: make(chan interface{}),
		state:    newState(StateInactive),
	}

	w.errBuffer = newErrBuffer(w.logCallback)

	// piping all stderr to command errBuffer
	w.cmd.Stderr = w.errBuffer

	return w, nil
}

// State return receive-only worker state object, state can be used to safely access
// worker status, time when status changed and number of worker executions.
func (w *Worker) State() State {
	return w.state
}

// String returns worker description.
func (w *Worker) String() string {
	state := w.state.String()
	if w.Pid != nil {
		state = state + ", pid:" + strconv.Itoa(*w.Pid)
	}

	return fmt.Sprintf(
		"(`%s` [%s], numExecs: %v)",
		strings.Join(w.cmd.Args, " "),
		state,
		w.state.NumExecs(),
	)
}

// Wait must be called once for each worker, call will be released once worker is
// complete and will return process error (if any), if stderr is presented it's value
// will be wrapped as WorkerError. Method will return error code if php process fails
// to find or start the script.
func (w *Worker) Wait() error {
	<-w.waitDone

	// ensure that all receive/send operations are complete
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.endState.Success() {
		w.state.set(StateStopped)
		return nil
	}

	if w.state.Value() != StateStopping {
		w.state.set(StateErrored)
	} else {
		w.state.set(StateStopped)
	}

	if w.errBuffer.Len() != 0 {
		return errors.New(w.errBuffer.String())
	}

	// generic process error
	return &exec.ExitError{ProcessState: w.endState}
}

// Stop sends soft termination command to the worker and waits for process completion.
func (w *Worker) Stop() error {
	select {
	case <-w.waitDone:
		return nil
	default:
		w.mu.Lock()
		defer w.mu.Unlock()

		w.state.set(StateStopping)
		err := sendControl(w.rl, &stopCommand{Stop: true})

		<-w.waitDone
		return err
	}
}

// Kill kills underlying process, make sure to call Wait() func to gather
// error log from the stderr. Does not waits for process completion!
func (w *Worker) Kill() error {
	select {
	case <-w.waitDone:
		return nil
	default:
		w.state.set(StateStopping)
		err := w.cmd.Process.Signal(os.Kill)

		<-w.waitDone
		return err
	}
}

func (w *Worker) start() error {
	if err := w.cmd.Start(); err != nil {
		close(w.waitDone)
		return err
	}

	w.Pid = &w.cmd.Process.Pid

	// wait for process to complete
	go func() {
		w.endState, _ = w.cmd.Process.Wait()
		if w.waitDone != nil {
			close(w.waitDone)
			w.mu.Lock()
			defer w.mu.Unlock()

			if w.rl != nil {
				err := w.rl.Close()
				if err != nil {
					w.Events <- WorkerEvent{Event: EventWorkerError, Worker: w, Payload: err}
				}
			}

			err := w.errBuffer.Close()
			if err != nil {
				w.Events <- WorkerEvent{Event: EventWorkerError, Worker: w, Payload: err}
			}

			// todo: check if we can merge waitDone and events
			close(w.Events)
		}
	}()

	return nil
}

func (w *Worker) logCallback(log []byte) {
	w.Events <- WorkerEvent{Event: EventWorkerLog, Worker: w, Payload: log}
}

// todo: do we still need it?
func (w *Worker) markInvalid() {
	w.state.set(StateInvalid)
}

// thread safe errBuffer
type errBuffer struct {
	mu          sync.Mutex
	buf         []byte
	last        int
	wait        *time.Timer
	update      chan interface{}
	stop        chan interface{}
	logCallback func(log []byte)
}

func newErrBuffer(logCallback func(log []byte)) *errBuffer {
	eb := &errBuffer{
		buf:         make([]byte, 0),
		update:      make(chan interface{}),
		wait:        time.NewTimer(WaitDuration),
		stop:        make(chan interface{}),
		logCallback: logCallback,
	}

	go func() {
		for {
			select {
			case <-eb.update:
				eb.wait.Reset(WaitDuration)
			case <-eb.wait.C:
				eb.mu.Lock()
				if len(eb.buf) > eb.last {
					eb.logCallback(eb.buf[eb.last:])
					eb.buf = eb.buf[0:0]
					eb.last = len(eb.buf)
				}
				eb.mu.Unlock()
			case <-eb.stop:
				eb.wait.Stop()

				eb.mu.Lock()
				if len(eb.buf) > eb.last {
					eb.logCallback(eb.buf[eb.last:])
					eb.last = len(eb.buf)
				}
				eb.mu.Unlock()
				return
			}
		}
	}()

	return eb
}

// Len returns the number of buf of the unread portion of the errBuffer;
// buf.Len() == len(buf.Bytes()).
func (eb *errBuffer) Len() int {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// currently active message
	return len(eb.buf)
}

// Write appends the contents of pool to the errBuffer, growing the errBuffer as
// needed. The return value n is the length of pool; errBuffer is always nil.
func (eb *errBuffer) Write(p []byte) (int, error) {
	eb.mu.Lock()
	eb.buf = append(eb.buf, p...)
	eb.mu.Unlock()
	eb.update <- nil

	return len(p), nil
}

// Strings fetches all errBuffer data into string.
func (eb *errBuffer) String() string {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	return string(eb.buf)
}

// Close aggregation timer.
func (eb *errBuffer) Close() error {
	close(eb.stop)
	return nil
}
