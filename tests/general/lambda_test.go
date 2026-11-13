package tests

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	configImpl "github.com/roadrunner-server/config/v6"
	"github.com/roadrunner-server/endure/v2"
	"github.com/roadrunner-server/informer/v6"
	"github.com/roadrunner-server/logger/v6"
	"github.com/roadrunner-server/resetter/v6"
	"github.com/roadrunner-server/rpc/v6"
	"github.com/roadrunner-server/server/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rrtemporal "github.com/temporalio/roadrunner-temporal/v6"
	"go.temporal.io/api/enums/v1"
	temporalClient "go.temporal.io/sdk/client"
)

const (
	drainActivitySeconds = 6
	// invocationBudget leaves a polling window of the budget minus the default
	// shutdown buffer, so the drain begins while the activity still runs.
	invocationBudget = time.Second * 9
)

// The plugin answers the Runtime API itself. Cycling the Temporal workers per
// invocation must not restart the PHP pools, and an invocation must not be
// answered while a PHP activity is still running: AWS freezes the execution
// environment right after the answer.
func Test_LambdaRuntime(t *testing.T) {
	bootLog := filepath.Join(t.TempDir(), "boots")
	t.Setenv("LAMBDA_BOOT_LOG", bootLog)
	api := startFakeRuntimeAPI(t)

	container := endure.New(slog.LevelError, endure.GracefulShutdownTimeout(time.Minute))

	require.NoError(t, container.RegisterAll(
		&configImpl.Plugin{Timeout: time.Minute, Path: "../configs/.rr-lambda.yaml", Version: "2025.1.11"},
		&rrtemporal.Plugin{},
		&logger.Plugin{},
		&resetter.Plugin{},
		&informer.Plugin{},
		&server.Plugin{},
		&rpc.Plugin{},
	))
	require.NoError(t, container.Init())

	_, err := container.Serve()
	require.NoError(t, err)

	defer func() {
		require.NoError(t, container.Stop())
	}()

	bootsAtStart := phpBoots(t, bootLog)
	require.NotEmpty(t, bootsAtStart, "no PHP worker was started at all")

	t.Run("the php pool survives the invocations", func(t *testing.T) {
		for range 3 {
			awaitAnswer(t, api.invoke(t, invocationBudget), time.Minute)
		}

		assert.Equal(
			t,
			bootsAtStart,
			phpBoots(t, bootLog),
			"a PHP worker was started again: the pool did not survive the invocations",
		)
	})

	t.Run("a running activity is drained before the answer", func(t *testing.T) {
		client, err := temporalClient.Dial(temporalClient.Options{HostPort: "127.0.0.1:7233", Namespace: "default"})
		require.NoError(t, err)
		defer client.Close()

		startedAt := time.Now()
		answered := api.invoke(t, invocationBudget)

		run, err := client.ExecuteWorkflow(
			context.Background(),
			temporalClient.StartWorkflowOptions{
				TaskQueue:                "default",
				WorkflowExecutionTimeout: time.Minute,
			},
			"LambdaSlowWorkflow",
			drainActivitySeconds,
		)
		require.NoError(t, err)

		awaitAnswer(t, answered, time.Minute)

		elapsed := time.Since(startedAt)
		t.Logf("activity %ds, the invocation was answered after %s", drainActivitySeconds, elapsed)

		assert.GreaterOrEqual(
			t,
			elapsed,
			time.Second*drainActivitySeconds,
			"the invocation was answered before the running PHP activity finished: it was abandoned, not drained",
		)

		assert.True(
			t,
			hasEvent(t, client, run, enums.EVENT_TYPE_ACTIVITY_TASK_COMPLETED),
			"the activity never completed: its work was dropped at the invocation boundary",
		)
	})
}

// phpBoots returns the pid of every PHP worker process the pools have started,
// one line per boot.
func phpBoots(t *testing.T, bootLog string) []string {
	t.Helper()

	recorded, err := os.ReadFile(bootLog)
	require.NoError(t, err)

	return strings.Fields(string(recorded))
}

func hasEvent(
	t *testing.T,
	client temporalClient.Client,
	run temporalClient.WorkflowRun,
	eventType enums.EventType,
) bool {
	t.Helper()

	iter := client.GetWorkflowHistory(
		context.Background(),
		run.GetID(),
		run.GetRunID(),
		false,
		enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT,
	)

	for iter.HasNext() {
		event, err := iter.Next()
		require.NoError(t, err)

		if event.GetEventType() == eventType {
			return true
		}
	}

	return false
}
