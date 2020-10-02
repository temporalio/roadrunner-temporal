package roadrunner

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/spiral/goridge/v2"
	"golang.org/x/sync/errgroup"
)

// SocketFactory connects to external workers using socket server.
type SocketFactory struct {
	// listens for incoming connections from underlying processes
	ls net.Listener

	// relay connection timeout
	tout time.Duration

	// sockets which are waiting for process association
	//relays map[int64]*goridge.SocketRelay
	relays sync.Map

	ErrCh chan error
}

// todo: review

// NewSocketServer returns SocketFactory attached to a given socket listener.
// tout specifies for how long factory should serve for incoming relay connection
func NewSocketServer(ls net.Listener, tout time.Duration) *SocketFactory {
	f := &SocketFactory{
		ls:     ls,
		tout:   tout,
		relays: sync.Map{}, //make(map[int64]*goridge.SocketRelay),
		ErrCh: make(chan error, 10),
	}

	// Be careful
	// https://github.com/go101/go101/wiki/About-memory-ordering-guarantees-made-by-atomic-operations-in-Go
	// https://github.com/golang/go/issues/5045
	go func() {
		f.ErrCh <- f.listen()
	}()

	return f
}

// blocking operation, returns an error
func (f *SocketFactory) listen() error {
	errGr := &errgroup.Group{}
	errGr.Go(func() error {
		for {
			conn, err := f.ls.Accept()
			if err != nil {
				return err
			}

			rl := goridge.NewSocketRelay(conn)
			pid, err := fetchPID(rl)
			if err != nil {
				return err
			}

			f.attachRelayToPid(pid, rl)
		}
	})

	return errGr.Wait()
}

// SpawnWorker creates WorkerProcess and connects it to appropriate relay or returns error
func (f *SocketFactory) SpawnWorker(ctx context.Context, cmd *exec.Cmd) (WorkerBase, error) {
	w, err := initWorker(ctx, cmd)
	if err != nil {
		return nil, err
	}

	err = w.Start(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "process error")
	}

	//go func(w WorkerBase) {
	//	if wErr := w.Wait(); wErr != nil {
	//		if _, ok := wErr.(*exec.ExitError); ok {
	//			err = errors.Wrap(wErr, err.Error())
	//		} else {
	//			err = wErr
	//		}
	//	}
	//}(w)

	rl, err := f.findRelay(ctx, w, f.tout)
	if err != nil {
		//go func(w WorkerBase) {
		//
		//}(w)
		err = w.Kill(ctx)
		if err != nil {
			fmt.Println(fmt.Errorf("error killing the WorkerProcess %v", err))
		}
		return nil, errors.Wrap(err, "unable to connect to WorkerProcess")
	}

	w.AttachRelay(ctx, rl)
	w.State(ctx).Set(StateReady)

	return w, nil
}

// Close socket factory and underlying socket connection.
func (f *SocketFactory) Close() error {
	return f.ls.Close()
}

// listens for incoming socket connections
// error will be reported to the pool events channel
//func (f *SocketFactory) listen() {
//	go func() {
//		for {
//			conn, err := f.ls.Accept()
//			if err != nil {
//				return err
//			}
//
//			rl := goridge.NewSocketRelay(conn)
//			pid, err := fetchPID(rl)
//			if err != nil {
//				return err
//			}
//
//			f.attachRelayToPid(pid, rl)
//
//			//if pid, err := fetchPID(rl); err == nil {
//			//	f.attachRelayToPid(int(pid)) <- rl
//			//}
//		}
//	}()
//}

// waits for WorkerProcess to connect over socket and returns associated relay of timeout
func (f *SocketFactory) findRelay(ctx context.Context, w WorkerBase, tout time.Duration) (*goridge.SocketRelay, error) {

	// destroy waiting for the relay after tout time
	timer := time.NewTimer(tout)
	// poll every 100ms for the relay
	pollTimer := time.NewTimer(time.Millisecond * 100)
	for {
		select {
		//case rl := <-f.attachRelayToPid(int(w.Pid()), nil):
		//	timer.Stop()
		//	f.removeRelayFromPid(int(w.Pid()))
		//	return rl, nil

		case <-timer.C:
			timer.Stop()
			return nil, fmt.Errorf("relay timeout")
		case <-pollTimer.C:
			tmp, ok := f.relays.Load(w.Pid(ctx))
			if !ok {
				continue
			}
			return tmp.(*goridge.SocketRelay), nil

			//case <-w.WaitChan(): // todo: to clean up waiting if worker dies
			//	timer.Stop()
			//	f.removeRelayFromPid(int(w.Pid()))
			//	return nil, fmt.Errorf("WorkerProcess is gone")
		}
	}
}

// chan to store relay associated with specific pid
func (f *SocketFactory) attachRelayToPid(pid int64, relay *goridge.SocketRelay) {
	f.relays.Store(pid, relay)
	//
	//rl, ok := f.relays[pid]
	//if !ok {
	//	f.relays[pid] = &goridge.SocketRelay{}
	//	return
	//}
	//f.relays[pid] = rl
}

// deletes relay chan associated with specific pid
func (f *SocketFactory) removeRelayFromPid(pid int64) {
	//f.mu.Lock()
	//defer f.mu.Unlock()
	//
	//delete(f.relays, pid)
	f.relays.Delete(pid)
}
