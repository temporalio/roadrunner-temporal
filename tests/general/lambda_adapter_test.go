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
)

// The lambda plugin drives the adapter once per invocation. Cycling the Temporal
// workers must not restart the PHP pools: the pools outlive every invocation.
func Test_LambdaAdapterKeepsThePhpPool(t *testing.T) {
	bootLog := filepath.Join(t.TempDir(), "boots")
	t.Setenv("AWS_LAMBDA_RUNTIME_API", "127.0.0.1:1")
	t.Setenv("LAMBDA_BOOT_LOG", bootLog)

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

	bootsAtStart := phpBoots(t, bootLog)
	require.NotEmpty(t, bootsAtStart, "no PHP worker was started at all")

	for range 3 {
		require.NoError(t, plugin.StartInvocation(context.Background()))
		time.Sleep(time.Millisecond * 200)
		require.NoError(t, plugin.StopInvocation(context.Background()))
	}

	require.NoError(t, plugin.StartInvocation(context.Background()))
	require.NoError(t, plugin.StopInvocation(context.Background()))

	assert.Equal(
		t,
		bootsAtStart,
		phpBoots(t, bootLog),
		"a PHP worker was started again: the pool did not survive the invocations",
	)
}

// phpBoots returns the pid of every PHP worker process the pools have started,
// one line per boot.
func phpBoots(t *testing.T, bootLog string) []string {
	t.Helper()

	recorded, err := os.ReadFile(bootLog)
	require.NoError(t, err)

	return strings.Fields(string(recorded))
}
