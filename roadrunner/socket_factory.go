package roadrunner

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/spiral/goridge/v2"
	"net"
	"os/exec"
	"sync"
	"time"
)

// SocketFactory connects to external workers using socket server.
type SocketFactory struct {
	// listens for incoming connections from underlying processes
	ls net.Listener

	// relay connection timeout
	tout time.Duration

	// protects socket mapping
	mu sync.Mutex

	// sockets which are waiting for process association
	relays map[int]chan *goridge.SocketRelay
}

// todo: review

// NewSocketFactory returns SocketFactory attached to a given socket listener.
// tout specifies for how long factory should serve for incoming relay connection
func NewSocketFactory(ls net.Listener, tout time.Duration) *SocketFactory {
	f := &SocketFactory{
		ls:     ls,
		tout:   tout,
		relays: make(map[int]chan *goridge.SocketRelay),
	}

	go f.listen()

	return f
}

// SpawnWorker creates WorkerProcess and connects it to appropriate relay or returns error
func (f *SocketFactory) SpawnWorker(cmd *exec.Cmd) (w Worker, err error) {
	if w, err = initWorker(cmd); err != nil {
		return nil, err
	}

	if err := w.Start(); err != nil {
		return nil, errors.Wrap(err, "process error")
	}

	rl, err := f.findRelay(w, f.tout)
	if err != nil {
		go func(w Worker) {
			err := w.Kill()
			if err != nil {
				fmt.Println(fmt.Errorf("error killing the WorkerProcess %v", err))
			}
		}(w)

		if wErr := w.Wait(); wErr != nil {
			if _, ok := wErr.(*exec.ExitError); ok {
				err = errors.Wrap(wErr, err.Error())
			} else {
				err = wErr
			}
		}

		return nil, errors.Wrap(err, "unable to connect to WorkerProcess")
	}

	w.AttachRelay(rl)
	w.State().Set(StateReady)

	return w, nil
}

// Close socket factory and underlying socket connection.
func (f *SocketFactory) Close() error {
	return f.ls.Close()
}

// listens for incoming socket connections
func (f *SocketFactory) listen() {
	for {
		conn, err := f.ls.Accept()
		if err != nil {
			return
		}

		rl := goridge.NewSocketRelay(conn)
		if pid, err := fetchPID(rl); err == nil {
			f.relayChan(pid) <- rl
		}
	}
}

// waits for WorkerProcess to connect over socket and returns associated relay of timeout
func (f *SocketFactory) findRelay(w Worker, tout time.Duration) (*goridge.SocketRelay, error) {
	timer := time.NewTimer(tout)
	for {
		select {
		case rl := <-f.relayChan(w.Pid()):
			timer.Stop()
			f.cleanChan(w.Pid())
			return rl, nil

		case <-timer.C:
			return nil, fmt.Errorf("relay timeout")

		case <-w.WaitChan(): // todo: to clean up waiting if worker dies
			timer.Stop()
			f.cleanChan(w.Pid())
			return nil, fmt.Errorf("WorkerProcess is gone")
		}
	}
}

// chan to store relay associated with specific pid
func (f *SocketFactory) relayChan(pid int) chan *goridge.SocketRelay {
	f.mu.Lock()
	defer f.mu.Unlock()

	rl, ok := f.relays[pid]
	if !ok {
		f.relays[pid] = make(chan *goridge.SocketRelay)
		return f.relays[pid]
	}

	return rl
}

// deletes relay chan associated with specific pid
func (f *SocketFactory) cleanChan(pid int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.relays, pid)
}
