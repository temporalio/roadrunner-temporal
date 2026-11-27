package rrtemporal

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/roadrunner-server/goridge/v4/pkg/frame"
	"github.com/roadrunner-server/pool/v2/fsm"
	"github.com/roadrunner-server/pool/v2/pool"
	"github.com/roadrunner-server/pool/v2/pool/static_pool"
	"github.com/roadrunner-server/pool/v2/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_STATUS_HELPER") != "1" {
		return
	}

	_, _ = io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}

type noopRelay struct{}

func (noopRelay) Send(*frame.Frame) error    { return nil }
func (noopRelay) Receive(*frame.Frame) error { return io.EOF }
func (noopRelay) Close() error               { return nil }

type statusTestFactory struct {
	state    int64
	cleanups *[]func()
}

func (f *statusTestFactory) SpawnWorkerWithContext(ctx context.Context, _ *exec.Cmd, opts ...worker.Options) (*worker.Process, error) {
	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, os.Args[0], "-test.run=^TestHelperProcess$") //nolint:gosec // Re-executes the test binary helper.
	cmd.Env = append(os.Environ(), "GO_WANT_STATUS_HELPER=1")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}

	w, err := worker.InitBaseWorker(cmd, opts...)
	if err != nil {
		_ = stdin.Close()
		cancel()
		return nil, err
	}

	w.AttachRelay(noopRelay{})
	if err := w.Start(); err != nil {
		_ = stdin.Close()
		cancel()
		return nil, err
	}

	switch f.state {
	case fsm.StateReady:
		w.State().Transition(fsm.StateReady)
	case fsm.StateWorking:
		w.State().Transition(fsm.StateReady)
		w.State().Transition(fsm.StateWorking)
	}

	*f.cleanups = append(*f.cleanups, func() {
		_ = stdin.Close()
		cancel()
	})

	return w, nil
}

func (f *statusTestFactory) Close() error { return nil }

func newStatusTestPool(t *testing.T, state int64) *static_pool.Pool {
	t.Helper()

	var cleanups []func()
	factory := &statusTestFactory{state: state, cleanups: &cleanups}

	sp, err := static_pool.NewPool(
		context.Background(),
		func([]string) *exec.Cmd { return exec.CommandContext(t.Context(), os.Args[0]) }, //nolint:gosec // Re-executes the test binary helper.
		factory,
		&pool.Config{
			NumWorkers:     1,
			DestroyTimeout: 2 * time.Second,
		},
		slog.New(slog.DiscardHandler),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		for _, fn := range cleanups {
			fn()
		}
		sp.Destroy(context.Background())
	})

	return sp
}

func TestPluginStatusAndReady(t *testing.T) {
	tests := []struct {
		name       string
		plugin     func(t *testing.T) *Plugin
		statusCode int
		readyCode  int
	}{
		{
			name:       "nil config and pools",
			plugin:     func(*testing.T) *Plugin { return &Plugin{} },
			statusCode: http.StatusServiceUnavailable,
			readyCode:  http.StatusServiceUnavailable,
		},
		{
			name:       "nil pools with activity workers enabled",
			plugin:     func(*testing.T) *Plugin { return &Plugin{config: &Config{}} },
			statusCode: http.StatusServiceUnavailable,
			readyCode:  http.StatusServiceUnavailable,
		},
		{
			name:       "nil pools with activity workers disabled",
			plugin:     func(*testing.T) *Plugin { return &Plugin{config: &Config{DisableActivityWorkers: true}} },
			statusCode: http.StatusServiceUnavailable,
			readyCode:  http.StatusServiceUnavailable,
		},
		{
			name: "inactive activity worker",
			plugin: func(t *testing.T) *Plugin {
				return &Plugin{
					config: &Config{},
					actP:   newStatusTestPool(t, fsm.StateInactive),
				}
			},
			statusCode: http.StatusServiceUnavailable,
			readyCode:  http.StatusServiceUnavailable,
		},
		{
			name: "ready activity worker",
			plugin: func(t *testing.T) *Plugin {
				return &Plugin{
					config: &Config{},
					actP:   newStatusTestPool(t, fsm.StateReady),
				}
			},
			statusCode: http.StatusOK,
			readyCode:  http.StatusOK,
		},
		{
			name: "working activity worker",
			plugin: func(t *testing.T) *Plugin {
				return &Plugin{
					config: &Config{},
					actP:   newStatusTestPool(t, fsm.StateWorking),
				}
			},
			statusCode: http.StatusOK,
			readyCode:  http.StatusOK,
		},
		{
			name: "working workflow worker with activity workers disabled",
			plugin: func(t *testing.T) *Plugin {
				return &Plugin{
					config: &Config{DisableActivityWorkers: true},
					wfP:    newStatusTestPool(t, fsm.StateWorking),
				}
			},
			statusCode: http.StatusOK,
			readyCode:  http.StatusOK,
		},
		{
			name: "inactive workflow worker with activity workers disabled and nil activity pool",
			plugin: func(t *testing.T) *Plugin {
				return &Plugin{
					config: &Config{DisableActivityWorkers: true},
					wfP:    newStatusTestPool(t, fsm.StateInactive),
				}
			},
			statusCode: http.StatusServiceUnavailable,
			readyCode:  http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.plugin(t)

			st, err := p.Status()
			require.NoError(t, err)
			assert.Equal(t, tt.statusCode, st.Code)

			rd, err := p.Ready()
			require.NoError(t, err)
			assert.Equal(t, tt.readyCode, rd.Code)
		})
	}
}
