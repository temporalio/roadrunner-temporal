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

	configImpl "github.com/roadrunner-server/config/v4"
	"github.com/roadrunner-server/endure/v2"
	"github.com/roadrunner-server/informer/v4"
	"github.com/roadrunner-server/logger/v4"
	"github.com/roadrunner-server/otel/v4"
	"github.com/roadrunner-server/resetter/v4"
	"github.com/roadrunner-server/rpc/v4"
	"github.com/roadrunner-server/server/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	roadrunnerTemporal "github.com/temporalio/roadrunner-temporal/v4"
	"github.com/temporalio/roadrunner-temporal/v4/data_converter"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	temporalClient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	rrPrefix  string = "rr"
	rrVersion string = "2023.3.0"
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

type log struct {
	zl *zap.Logger
}

// NewZapAdapter ... which uses general log interface
func newZapAdapter(zapLogger *zap.Logger) *log {
	return &log{
		zl: zapLogger.WithOptions(zap.AddCallerSkip(1)),
	}
}

func (l *log) Debug(msg string, keyvals ...any) {
	l.zl.Debug(msg, l.fields(keyvals)...)
}

func (l *log) Info(msg string, keyvals ...any) {
	l.zl.Info(msg, l.fields(keyvals)...)
}

func (l *log) Warn(msg string, keyvals ...any) {
	l.zl.Warn(msg, l.fields(keyvals)...)
}

func (l *log) Error(msg string, keyvals ...any) {
	l.zl.Error(msg, l.fields(keyvals)...)
}

func (l *log) fields(keyvals []any) []zap.Field {
	// we should have an even number of keys and values
	if len(keyvals)%2 != 0 {
		return []zap.Field{zap.Error(fmt.Errorf("odd number of keyvals pairs: %v", keyvals))}
	}

	zf := make([]zap.Field, len(keyvals)/2)
	j := 0
	for i := 0; i < len(keyvals); i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", keyvals[i])
		}

		zf[j] = zap.Any(key, keyvals[i+1])
		j++
	}

	return zf
}

func NewTestServer(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup, configPath string) *TestServer {
	container := endure.New(slog.LevelDebug, endure.GracefulShutdownTimeout(time.Second*30))

	cfg := &configImpl.Plugin{
		Timeout: time.Second * 30,
		Path:    configPath,
		Prefix:  rrPrefix,
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

	dc := data_converter.NewDataConverter(converter.GetDefaultDataConverter())
	client, err := temporalClient.Dial(temporalClient.Options{
		HostPort:      "127.0.0.1:7233",
		Namespace:     "default",
		DataConverter: dc,
		Logger:        newZapAdapter(initLogger()),
	})
	if err != nil {
		t.Fatal(err)
	}

	return &TestServer{
		Client: client,
	}
}

func NewTestServerTLS(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup, configName string) *TestServer {
	container := endure.New(slog.LevelDebug, endure.GracefulShutdownTimeout(time.Second*30))

	cfg := &configImpl.Plugin{
		Timeout: time.Second * 30,
		Path:    "../configs/tls/" + configName,
		Prefix:  rrPrefix,
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

	dc := data_converter.NewDataConverter(converter.GetDefaultDataConverter())

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
		Logger:        newZapAdapter(initLogger()),
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

func NewTestServerWithInterceptor(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup) *TestServer {
	container := endure.New(slog.LevelDebug, endure.GracefulShutdownTimeout(time.Second*30))

	cfg := &configImpl.Plugin{
		Timeout: time.Second * 30,
	}
	cfg.Path = "../configs/.rr-proto.yaml"
	cfg.Prefix = rrPrefix
	cfg.Version = rrVersion

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

	assert.NoError(t, err)
	assert.NoError(t, container.Init())

	errCh, err := container.Serve()
	if err != nil {
		panic(err)
	}

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

	dc := data_converter.NewDataConverter(converter.GetDefaultDataConverter())
	client, err := temporalClient.Dial(temporalClient.Options{
		HostPort:      "127.0.0.1:7233",
		Namespace:     "default",
		DataConverter: dc,
		Logger:        newZapAdapter(initLogger()),
	})
	if err != nil {
		panic(err)
	}
	require.NoError(t, err)

	return &TestServer{
		Client: client,
	}
}

func NewTestServerWithOtelInterceptor(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup) *TestServer {
	container := endure.New(slog.LevelDebug, endure.GracefulShutdownTimeout(time.Second*30))

	cfg := &configImpl.Plugin{
		Timeout: time.Second * 30,
	}
	cfg.Path = "configs/.rr-otlp.yaml"
	cfg.Prefix = rrPrefix
	cfg.Version = rrVersion

	err := container.RegisterAll(
		cfg,
		&roadrunnerTemporal.Plugin{},
		&logger.Plugin{},
		&resetter.Plugin{},
		&informer.Plugin{},
		&server.Plugin{},
		&rpc.Plugin{},
		&otel.Plugin{},
	)

	assert.NoError(t, err)
	assert.NoError(t, container.Init())

	errCh, err := container.Serve()
	if err != nil {
		panic(err)
	}

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

	dc := data_converter.NewDataConverter(converter.GetDefaultDataConverter())
	client, err := temporalClient.Dial(temporalClient.Options{
		HostPort:      "127.0.0.1:7233",
		Namespace:     "default",
		DataConverter: dc,
		Logger:        newZapAdapter(initLogger()),
	})
	if err != nil {
		panic(err)
	}
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

func initLogger() *zap.Logger {
	cfg := zap.Config{
		Level:    zap.NewAtomicLevelAt(zap.ErrorLevel),
		Encoding: "console",
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:    "message",
			LevelKey:      "level",
			TimeKey:       "time",
			CallerKey:     "caller",
			NameKey:       "name",
			StacktraceKey: "stack",
			EncodeLevel:   zapcore.CapitalLevelEncoder,
			EncodeTime:    zapcore.ISO8601TimeEncoder,
			EncodeCaller:  zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	l, err := cfg.Build(zap.AddCaller())
	if err != nil {
		panic(err)
	}

	return l
}
