package tests

import (
	"context"
	"log/slog"
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
	gracefulBudget       = time.Second * 20
)

// headStart is how long the activity is left running before the invocation is
// stopped, so the expected drain is drainActivitySeconds minus this.
var headStart = time.Second

// StopInvocation must not return while a PHP activity is still running: the
// lambda plugin answers the Runtime API right after it, and AWS then freezes
// the execution environment with that activity suspended mid-call.
func Test_LambdaStopInvocationDrainsRunningActivity(t *testing.T) {
	t.Setenv("AWS_LAMBDA_RUNTIME_API", "127.0.0.1:1")

	plugin := &rrtemporal.Plugin{}
	container := endure.New(slog.LevelError, endure.GracefulShutdownTimeout(time.Minute))

	require.NoError(t, container.RegisterAll(
		&configImpl.Plugin{Timeout: time.Minute, Path: "../configs/.rr-lambda.yaml", Version: "2025.1.11"},
		plugin,
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

	client, err := temporalClient.Dial(temporalClient.Options{HostPort: "127.0.0.1:7233", Namespace: "default"})
	require.NoError(t, err)
	defer client.Close()

	require.NoError(t, plugin.StartInvocation(context.Background(), gracefulBudget))

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

	waitForActivityScheduled(t, client, run)

	drainCtx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	startedAt := time.Now()
	require.NoError(t, plugin.StopInvocation(drainCtx))
	elapsed := time.Since(startedAt)

	t.Logf("head start %s, activity %ds, StopInvocation waited %s", headStart, drainActivitySeconds, elapsed)

	// Stop() has to wait for what is left of the activity, not for a fixed time.
	expected := time.Second*drainActivitySeconds - headStart - time.Second

	assert.GreaterOrEqual(
		t,
		elapsed,
		expected,
		"StopInvocation did not wait for the running PHP activity: it was abandoned, not drained",
	)

	assert.True(
		t,
		hasEvent(t, client, run, enums.EVENT_TYPE_ACTIVITY_TASK_COMPLETED),
		"the activity never completed: its work was dropped at the invocation boundary",
	)
}

func waitForActivityScheduled(t *testing.T, client temporalClient.Client, run temporalClient.WorkflowRun) {
	t.Helper()

	deadline := time.Now().Add(time.Second * 20)
	for time.Now().Before(deadline) {
		if hasEvent(t, client, run, enums.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED) {
			// the scheduled event is written before the worker picks the task up
			time.Sleep(headStart)

			return
		}

		time.Sleep(time.Millisecond * 200)
	}

	t.Fatal("the activity was never scheduled")
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
