package helpers

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	mocklogger "tests/mock"

	"github.com/roadrunner-server/status/v6"

	configImpl "github.com/roadrunner-server/config/v6"
	"github.com/roadrunner-server/endure/v2"
	"github.com/roadrunner-server/informer/v6"
	"github.com/roadrunner-server/logger/v6"
	"github.com/roadrunner-server/resetter/v6"
	"github.com/roadrunner-server/rpc/v6"
	"github.com/roadrunner-server/server/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	roadrunnerTemporal "github.com/temporalio/roadrunner-temporal/v6"
	"github.com/temporalio/roadrunner-temporal/v6/dataconverter"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	temporalClient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	sdklog "go.temporal.io/sdk/log"
)

const (
	rrVersion string = "2025.1.11"
)

type Configurer interface {
	// UnmarshalKey takes a single key and unmarshal it into a Struct.
	UnmarshalKey(name string, out any) error
	// Has checks if a config section exists.
	Has(name string) bool
}

type TestServer struct {
	Client temporalClient.Client
}

func NewTestServer(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup, configPath string) *TestServer {
	startContainer(t, stopCh, wg, configPath, &logger.Plugin{}, &status.Plugin{})
	return dial(t, temporalClient.ConnectionOptions{})
}

func NewTestServerTLS(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup, configName string) *TestServer {
	startContainer(t, stopCh, wg, "../configs/tls/"+configName, &logger.Plugin{})

	cert, err := tls.LoadX509KeyPair("../env/temporal_tls/certs/client.pem", "../env/temporal_tls/certs/client.key")
	require.NoError(t, err)

	certPool, err := x509.SystemCertPool()
	require.NoError(t, err)

	if certPool == nil {
		certPool = x509.NewCertPool()
	}

	rca, err := os.ReadFile("../env/temporal_tls/certs/ca.cert")
	require.NoError(t, err)

	if ok := certPool.AppendCertsFromPEM(rca); !ok {
		t.Fatal("appendCertsFromPEM")
	}

	return dial(t, temporalClient.ConnectionOptions{
		TLS: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
			RootCAs:      certPool,
			ServerName:   "tls-sample",
		},
	})
}

func NewTestServerWithInterceptor(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup, configPath string) *TestServer {
	startContainer(t, stopCh, wg, configPath, &logger.Plugin{}, &TemporalInterceptorPlugin{})
	return dial(t, temporalClient.ConnectionOptions{})
}

func NewTestServerWithDataConverter(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup) *TestServer {
	startContainer(t, stopCh, wg, "../configs/.rr-data-converter.yaml", &logger.Plugin{}, &TestDataConverterPlugin{})
	return dial(t, temporalClient.ConnectionOptions{})
}

func NewTestServerWithOtelInterceptor(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup) (*TestServer, *InMemoryOtelInterceptorPlugin) {
	otelPlugin := NewInMemoryOtelInterceptorPlugin(t)
	startContainer(t, stopCh, wg, "../configs/.rr-proto.yaml", &logger.Plugin{}, otelPlugin)
	return dial(t, temporalClient.ConnectionOptions{}), otelPlugin
}

func NewTestServerWithLogObserver(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup, configPath string) *mocklogger.ObservedLogs {
	l, oLogger := mocklogger.SlogTestLogger(slog.LevelDebug)
	startContainer(t, stopCh, wg, configPath, l)
	return oLogger
}

// startContainer registers, initializes and serves the base plugins and the extra plugins.
// It calls wg.Done after the container stops on a vertex error or on stopCh.
func startContainer(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup, cfgPath string, plugins ...any) {
	container := endure.New(slog.LevelDebug, endure.GracefulShutdownTimeout(time.Minute))

	cfg := &configImpl.Plugin{
		Timeout: time.Minute,
		Path:    cfgPath,
		Version: rrVersion,
	}

	err := container.RegisterAll(append([]any{
		cfg,
		&roadrunnerTemporal.Plugin{},
		&resetter.Plugin{},
		&informer.Plugin{},
		&server.Plugin{},
		&rpc.Plugin{},
	}, plugins...)...)
	require.NoError(t, err)
	require.NoError(t, container.Init())

	errCh, err := container.Serve()
	require.NoError(t, err)

	go func() {
		defer wg.Done()
		select {
		case er := <-errCh:
			assert.Fail(t, fmt.Sprintf("got error from vertex: %s, error: %v", er.VertexID, er.Error))
		case <-stopCh:
		}
		assert.NoError(t, container.Stop())
	}()
}

func dial(t *testing.T, co temporalClient.ConnectionOptions) *TestServer {
	client, err := temporalClient.Dial(temporalClient.Options{
		HostPort:          "127.0.0.1:7233",
		Namespace:         "default",
		DataConverter:     dataconverter.NewDataConverter(converter.GetDefaultDataConverter()),
		Logger:            sdklog.NewStructuredLogger(initLogger()),
		ConnectionOptions: co,
	})
	require.NoError(t, err)

	return &TestServer{
		Client: client,
	}
}

func (s *TestServer) AssertContainsEvent(client temporalClient.Client, t *testing.T, w temporalClient.WorkflowRun, assert func(*history.HistoryEvent) bool) {
	i := client.GetWorkflowHistory(
		context.Background(),
		w.GetID(),
		w.GetRunID(),
		false,
		enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT,
	)

	for {
		if !i.HasNext() {
			t.Error("no more events and no match found")
			break
		}

		e, err := i.Next()
		if err != nil {
			t.Error("unable to read history event")
			break
		}

		if assert(e) {
			break
		}
	}
}

func (s *TestServer) AssertNotContainsEvent(client temporalClient.Client, t *testing.T, w temporalClient.WorkflowRun, assert func(*history.HistoryEvent) bool) {
	i := client.GetWorkflowHistory(
		context.Background(),
		w.GetID(),
		w.GetRunID(),
		false,
		enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT,
	)

	for i.HasNext() {
		e, err := i.Next()
		if err != nil {
			t.Error("unable to read history event")
			break
		}

		if assert(e) {
			t.Error("found unexpected event")
			break
		}
	}
}

func initLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
}
