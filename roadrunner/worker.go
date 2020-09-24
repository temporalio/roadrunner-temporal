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

// EventWorkerKill thrown after WorkerProcess is being forcefully killed.
const (
	// EventWorkerError triggered after WorkerProcess. Except payload to be error.
	EventWorkerError = iota + 100

	// EventWorkerLog triggered on every write to WorkerProcess StdErr pipe (batched). Except payload to be []byte string.
	EventWorkerLog

	// WaitDuration - for how long error buffer should attempt to aggregate error messages
	// before merging output together since lastError update (required to keep error update together).
	WaitDuration = 100 * time.Millisecond
)

// todo: write comment
type WorkerEvent struct {
	Event   int
	Worker  *WorkerProcess
	Payload interface{}
}

type Worker interface {
	fmt.Stringer

	Created() time.Time

	Events() <-chan WorkerEvent

	Pid() int

	// State return receive-only WorkerProcess state object, state can be used to safely access
	// WorkerProcess status, time when status changed and number of WorkerProcess executions.
	State() State

	Start() error

	// Wait must be called once for each WorkerProcess, call will be released once WorkerProcess is
	// complete and will return process error (if any), if stderr is presented it's value
	// will be wrapped as WorkerError. Method will return error code if php process fails
	// to find or Start the script.
	Wait() error

	// Closed when worker stops. // todo: see socket factory, see static pool
	WaitChan() chan interface{}

	// Stop sends soft termination command to the WorkerProcess and waits for process completion.
	Stop() error

	// Kill kills underlying process, make sure to call Wait() func to gather
	// error log from the stderr. Does not waits for process completion!
	Kill() error

	Relay() goridge.Relay
	AttachRelay(rl goridge.Relay)
}

// WorkerProcess - supervised process with api over goridge.Relay.
type WorkerProcess struct {
	// created indicates at what time WorkerProcess has been created.
	created time.Time

	// updates parent supervisor or pool about WorkerProcess events
	events chan WorkerEvent

	// state holds information about current WorkerProcess state,
	// number of WorkerProcess executions, buf status change time.
	// publicly this object is receive-only and protected using Mutex
	// and atomic counter.
	state *state

	// underlying command with associated process, command must be
	// provided to WorkerProcess from outside in non-started form. CmdSource
	// stdErr direction will be handled by WorkerProcess to aggregate error message.
	cmd *exec.Cmd

	// pid of the process, points to pid of underlying process and
	// can be nil while process is not started.
	pid *int // todo: drop it?

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
	relay goridge.Relay
}

// initWorker creates new WorkerProcess over given exec.cmd.
func initWorker(cmd *exec.Cmd) (Worker, error) {
	if cmd.Process != nil {
		return nil, fmt.Errorf("can't attach to running process")
	}

	w := &WorkerProcess{
		created:  time.Now(),
		events:   make(chan WorkerEvent),
		cmd:      cmd,
		waitDone: make(chan interface{}), // todo: deprecate?
		state:    newState(StateInactive),
	}

	w.errBuffer = newErrBuffer(w.logCallback)

	// piping all stderr to command errBuffer
	w.cmd.Stderr = w.errBuffer

	return w, nil
}

func (w *WorkerProcess) Created() time.Time {
	return w.created
}

func (w *WorkerProcess) Events() <-chan WorkerEvent {
	return w.events
}

func (w *WorkerProcess) Pid() int64 {
	return *w.pid
}

// State return receive-only WorkerProcess state object, state can be used to safely access
// WorkerProcess status, time when status changed and number of WorkerProcess executions.
func (w *WorkerProcess) State() State {
	return w.state
}

// State return receive-only WorkerProcess state object, state can be used to safely access
// WorkerProcess status, time when status changed and number of WorkerProcess executions.
func (w *WorkerProcess) AttachRelay(rl goridge.Relay) {
	w.relay = rl
}

// State return receive-only WorkerProcess state object, state can be used to safely access
// WorkerProcess status, time when status changed and number of WorkerProcess executions.
func (w *WorkerProcess) Relay() goridge.Relay {
	return w.relay
}

// String returns WorkerProcess description.
func (w *WorkerProcess) String() string {
	state := w.state.String()
	if w.pid != nil {
		state = state + ", pid:" + strconv.Itoa(*w.pid)
	}

	return fmt.Sprintf(
		"(`%s` [%s], numExecs: %v)",
		strings.Join(w.cmd.Args, " "),
		state,
		w.state.NumExecs(),
	)
}

func (w *WorkerProcess) Start() error {
	if err := w.cmd.Start(); err != nil {
		close(w.waitDone)
		return err
	}

	w.pid = &w.cmd.Process.Pid

	// wait for process to complete
	go func() {
		w.endState, _ = w.cmd.Process.Wait()
		if w.waitDone != nil {
			close(w.waitDone)
			w.mu.Lock()
			defer w.mu.Unlock()

			if w.relay != nil {
				err := w.relay.Close()
				if err != nil {
					w.events <- WorkerEvent{Event: EventWorkerError, Worker: w, Payload: err}
				}
			}

			err := w.errBuffer.Close()
			if err != nil {
				w.events <- WorkerEvent{Event: EventWorkerError, Worker: w, Payload: err}
			}

			// todo: check if we can merge waitDone and events
			close(w.events)
		}
	}()

	return nil
}

// Wait must be called once for each WorkerProcess, call will be released once WorkerProcess is
// complete and will return process error (if any), if stderr is presented it's value
// will be wrapped as WorkerError. Method will return error code if php process fails
// to find or Start the script.
func (w *WorkerProcess) Wait() error {
	<-w.waitDone

	// ensure that all receive/send operations are complete
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.endState.Success() {
		w.state.Set(StateStopped)
		return nil
	}

	if w.state.Value() != StateStopping {
		w.state.Set(StateErrored)
	} else {
		w.state.Set(StateStopped)
	}

	if w.errBuffer.Len() != 0 {
		return errors.New(w.errBuffer.String())
	}

	// generic process error
	return &exec.ExitError{ProcessState: w.endState}
}

func (w *WorkerProcess) WaitChan() chan interface{} {
	return w.waitDone
}

// Stop sends soft termination command to the WorkerProcess and waits for process completion.
func (w *WorkerProcess) Stop() error {
	select {
	case <-w.waitDone:
		return nil
	default:
		w.mu.Lock()
		defer w.mu.Unlock()

		w.state.Set(StateStopping)
		err := sendControl(w.relay, &stopCommand{Stop: true})

		<-w.waitDone
		return err
	}
}

// Kill kills underlying process, make sure to call Wait() func to gather
// error log from the stderr. Does not waits for process completion!
func (w *WorkerProcess) Kill() error {
	select {
	case <-w.waitDone:
		return nil
	default:
		w.state.Set(StateStopping)
		err := w.cmd.Process.Signal(os.Kill)

		<-w.waitDone
		return err
	}
}

func (w *WorkerProcess) logCallback(log []byte) {
	w.events <- WorkerEvent{Event: EventWorkerLog, Worker: w, Payload: log}
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
