package aggregatedpool

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/temporalio/roadrunner-temporal/v6/internal"
	temporalClient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func TestTemporalWorkersKeepWorkerStopTimeout(t *testing.T) {
	client, err := temporalClient.NewLazyClient(temporalClient.Options{})
	require.NoError(t, err)

	info := &internal.WorkerInfo{
		TaskQueue: "default",
		Options:   worker.Options{WorkerStopTimeout: 5 * time.Second},
	}

	_, err = TemporalWorkers(
		&Workflow{},
		&Activity{},
		[]*internal.WorkerInfo{info},
		slog.New(slog.DiscardHandler),
		client,
		nil,
		nil,
	)
	require.NoError(t, err)

	require.Equal(t, 5*time.Second, info.Options.WorkerStopTimeout)
}
