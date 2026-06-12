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
	tlog "go.temporal.io/sdk/log"
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
	container := endure.New(slog.LevelDebug, endure.GracefulShutdownTimeout(time.Minute))

	cfg := &configImpl.Plugin{
		Timeout: time.Minute,
		Path:    configPath,
		Version: rrVersion,
	}

	err := container.RegisterAll(
		cfg,
		&roadrunnerTemporal.Plugin{},
		&logger.Plugin{},
		&resetter.Plugin{},
		&informer.Plugin{},
		&server.Plugin{},
		&rpc.Plugin{},
		&status.Plugin{},
	)

	require.NoError(t, err)
	require.NoError(t, container.Init())

	errCh, err := container.Serve()
	require.NoError(t, err)

	go func() {
		defer wg.Done()
		for {
			select {
			case er := <-errCh:
				assert.Fail(t, fmt.Sprintf("got error from vertex: %s, error: %v", er.VertexID, er.Error))
				require.NoError(t, container.Stop())
				return
			case <-stopCh:
				require.NoError(t, container.Stop())
				return
			}
		}
	}()

	dc := dataconverter.NewDataConverter(converter.GetDefaultDataConverter())
	client, err := temporalClient.Dial(temporalClient.Options{
		HostPort:      "127.0.0.1:7233",
		Namespace:     "default",
		DataConverter: dc,
		Logger:        tlog.NewStructuredLogger(initLogger()),
	})
	if err != nil {
		t.Fatal(err)
	}

	return &TestServer{
		Client: client,
	}
}

func NewTestServerTLS(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup, configName string) *TestServer {
	container := endure.New(slog.LevelDebug, endure.GracefulShutdownTimeout(time.Minute))

	cfg := &configImpl.Plugin{
		Timeout: time.Minute,
		Path:    "../configs/tls/" + configName,
		Version: rrVersion,
	}

	err := container.RegisterAll(
		cfg,
		&roadrunnerTemporal.Plugin{},
		&logger.Plugin{},
		&resetter.Plugin{},
		&informer.Plugin{},
		&server.Plugin{},
		&rpc.Plugin{},
	)

	assert.NoError(t, err)
	assert.NoError(t, container.Init())

	errCh, err := container.Serve()
	require.NoError(t, err)

	go func() {
		defer wg.Done()
		for {
			select {
			case er := <-errCh:
				assert.Fail(t, fmt.Sprintf("got error from vertex: %s, error: %v", er.VertexID, er.Error))
				assert.NoError(t, container.Stop())
				return
			case <-stopCh:
				assert.NoError(t, container.Stop())
				return
			}
		}
	}()

	dc := dataconverter.NewDataConverter(converter.GetDefaultDataConverter())

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

	client, err := temporalClient.Dial(temporalClient.Options{
		HostPort:      "127.0.0.1:7233",
		Namespace:     "default",
		DataConverter: dc,
		Logger:        tlog.NewStructuredLogger(initLogger()),
		ConnectionOptions: temporalClient.ConnectionOptions{
			TLS: &tls.Config{
				MinVersion:   tls.VersionTLS12,
				ClientAuth:   tls.RequireAndVerifyClientCert,
				Certificates: []tls.Certificate{cert},
				ClientCAs:    certPool,
				RootCAs:      certPool,
				ServerName:   "tls-sample",
			},
		},
	})
	require.NoError(t, err)

	return &TestServer{
		Client: client,
	}
}

func NewTestServerWithInterceptor(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup, configPaths ...string) *TestServer {
	container := endure.New(slog.LevelDebug, endure.GracefulShutdownTimeout(time.Minute))

	cfgPath := "../configs/.rr-proto.yaml"
	if len(configPaths) > 0 {
		cfgPath = configPaths[0]
	}

	cfg := &configImpl.Plugin{
		Timeout: time.Minute,
		Path:    cfgPath,
		Version: rrVersion,
	}

	err := container.RegisterAll(
		cfg,
		&roadrunnerTemporal.Plugin{},
		&logger.Plugin{},
		&resetter.Plugin{},
		&informer.Plugin{},
		&server.Plugin{},
		&rpc.Plugin{},
		&TemporalInterceptorPlugin{},
	)

	require.NoError(t, err)
	require.NoError(t, container.Init())

	errCh, err := container.Serve()
	require.NoError(t, err)

	go func() {
		defer wg.Done()
		for {
			select {
			case er := <-errCh:
				assert.Fail(t, fmt.Sprintf("got error from vertex: %s, error: %v", er.VertexID, er.Error))
				assert.NoError(t, container.Stop())
				return
			case <-stopCh:
				assert.NoError(t, container.Stop())
				return
			}
		}
	}()

	dc := dataconverter.NewDataConverter(converter.GetDefaultDataConverter())
	client, err := temporalClient.Dial(temporalClient.Options{
		HostPort:      "127.0.0.1:7233",
		Namespace:     "default",
		DataConverter: dc,
		Logger:        tlog.NewStructuredLogger(initLogger()),
	})
	require.NoError(t, err)

	return &TestServer{
		Client: client,
	}
}

func NewTestServerWithDataConverter(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup, configPaths ...string) *TestServer {
	container := endure.New(slog.LevelDebug, endure.GracefulShutdownTimeout(time.Minute))

	cfgPath := "../configs/.rr-data-converter.yaml"
	if len(configPaths) > 0 {
		cfgPath = configPaths[0]
	}

	cfg := &configImpl.Plugin{
		Timeout: time.Minute,
		Path:    cfgPath,
		Version: rrVersion,
	}

	err := container.RegisterAll(
		cfg,
		&roadrunnerTemporal.Plugin{},
		&logger.Plugin{},
		&resetter.Plugin{},
		&informer.Plugin{},
		&server.Plugin{},
		&rpc.Plugin{},
		&TestDataConverterPlugin{},
	)

	require.NoError(t, err)
	require.NoError(t, container.Init())

	errCh, err := container.Serve()
	require.NoError(t, err)

	go func() {
		defer wg.Done()
		for {
			select {
			case er := <-errCh:
				assert.Fail(t, fmt.Sprintf("got error from vertex: %s, error: %v", er.VertexID, er.Error))
				assert.NoError(t, container.Stop())
				return
			case <-stopCh:
				assert.NoError(t, container.Stop())
				return
			}
		}
	}()

	dc := dataconverter.NewDataConverter(converter.GetDefaultDataConverter())
	client, err := temporalClient.Dial(temporalClient.Options{
		HostPort:      "127.0.0.1:7233",
		Namespace:     "default",
		DataConverter: dc,
		Logger:        tlog.NewStructuredLogger(initLogger()),
	})
	require.NoError(t, err)

	return &TestServer{
		Client: client,
	}
}

func NewTestServerWithOtelInterceptor(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup) (*TestServer, *InMemoryOtelInterceptorPlugin) {
	container := endure.New(slog.LevelDebug, endure.GracefulShutdownTimeout(time.Minute))

	otelPlugin := NewInMemoryOtelInterceptorPlugin(t)

	cfg := &configImpl.Plugin{
		Timeout: time.Minute,
		Path:    "../configs/.rr-proto.yaml",
		Version: rrVersion,
	}

	err := container.RegisterAll(
		cfg,
		&roadrunnerTemporal.Plugin{},
		&logger.Plugin{},
		&resetter.Plugin{},
		&informer.Plugin{},
		&server.Plugin{},
		&rpc.Plugin{},
		otelPlugin,
	)

	require.NoError(t, err)
	require.NoError(t, container.Init())

	errCh, err := container.Serve()
	require.NoError(t, err)

	go func() {
		defer wg.Done()
		for {
			select {
			case er := <-errCh:
				assert.Fail(t, fmt.Sprintf("got error from vertex: %s, error: %v", er.VertexID, er.Error))
				assert.NoError(t, container.Stop())
				return
			case <-stopCh:
				assert.NoError(t, container.Stop())
				return
			}
		}
	}()

	dc := dataconverter.NewDataConverter(converter.GetDefaultDataConverter())
	client, err := temporalClient.Dial(temporalClient.Options{
		HostPort:      "127.0.0.1:7233",
		Namespace:     "default",
		DataConverter: dc,
		Logger:        tlog.NewStructuredLogger(initLogger()),
	})
	require.NoError(t, err)

	return &TestServer{
		Client: client,
	}, otelPlugin
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
		Level:     slog.LevelError,
		AddSource: true,
	}))
}
