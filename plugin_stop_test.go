package rrtemporal

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/roadrunner-server/events"
	"github.com/stretchr/testify/require"
	"github.com/temporalio/roadrunner-temporal/v6/internal"
	"go.temporal.io/sdk/worker"
)

const stopTestTimeout = 5 * time.Second

type blockingWorker struct {
	worker.Worker

	stopping chan struct{}
	release  chan struct{}
}

func (b *blockingWorker) Stop() {
	close(b.stopping)
	<-b.release
}

type drainingPlugin struct {
	plugin  *Plugin
	release func()
	stopped chan error
}

// startDraining leaves Stop blocked inside the shutdown, holding p.mu, which is
// where a heartbeat from a still-running activity arrives.
func startDraining(t *testing.T) *drainingPlugin {
	t.Helper()

	w := &blockingWorker{
		stopping: make(chan struct{}),
		release:  make(chan struct{}),
	}

	p := &Plugin{
		log:    slog.New(slog.DiscardHandler),
		stopCh: make(chan struct{}, 1),
		temporal: &temporal{
			activities: map[string]*internal.ActivityInfo{},
			workflows:  map[string]*internal.WorkflowInfo{},
			workers:    []worker.Worker{w},
		},
	}
	p.eventBus, p.id = events.NewEventBus()

	var once sync.Once
	release := func() { once.Do(func() { close(w.release) }) }
	t.Cleanup(release)

	stopped := make(chan error, 1)
	go func() { stopped <- p.Stop(context.Background()) }()

	select {
	case <-w.stopping:
	case <-time.After(stopTestTimeout):
		require.FailNow(t, "Stop did not reach the worker drain")
	}

	return &drainingPlugin{plugin: p, release: release, stopped: stopped}
}

func (d *drainingPlugin) finish(t *testing.T) {
	t.Helper()

	d.release()

	select {
	case err := <-d.stopped:
		require.NoError(t, err)
	case <-time.After(stopTestTimeout):
		require.FailNow(t, "Stop did not return after the drain finished")
	}
}

func TestDefinitionsAreReadableWhileStopping(t *testing.T) {
	d := startDraining(t)

	done := make(chan struct{})
	go func() {
		d.plugin.getActDef()
		d.plugin.getWfDef()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(stopTestTimeout):
		require.FailNow(t, "reading the activity definition blocked while the plugin was stopping")
	}

	d.finish(t)
}
