package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/roadrunner-server/api/v2/plugins/config"
	configImpl "github.com/roadrunner-server/config/v2"
	endure "github.com/roadrunner-server/endure/pkg/container"
	"github.com/roadrunner-server/informer/v2"
	"github.com/roadrunner-server/logger/v2"
	"github.com/roadrunner-server/resetter/v2"
	"github.com/roadrunner-server/rpc/v2"
	"github.com/roadrunner-server/server/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	roadrunnerTemporal "github.com/temporalio/roadrunner-temporal"
	"github.com/temporalio/roadrunner-temporal/internal/data_converter"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	temporalClient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type TestServer struct {
	client temporalClient.Client
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

func (l *log) Debug(msg string, keyvals ...interface{}) {
	l.zl.Debug(msg, l.fields(keyvals)...)
}

func (l *log) Info(msg string, keyvals ...interface{}) {
	l.zl.Info(msg, l.fields(keyvals)...)
}

func (l *log) Warn(msg string, keyvals ...interface{}) {
	l.zl.Warn(msg, l.fields(keyvals)...)
}

func (l *log) Error(msg string, keyvals ...interface{}) {
	l.zl.Error(msg, l.fields(keyvals)...)
}

func (l *log) fields(keyvals []interface{}) []zap.Field {
	// we should have even number of keys and values
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

func NewTestServerWithMetrics(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup) *TestServer {
	container, err := endure.NewContainer(initLogger(), endure.RetryOnFail(false))
	assert.NoError(t, err)

	err = container.RegisterAll(
		&roadrunnerTemporal.Plugin{},
		initConfigProtoWithMetrics(),
		&logger.Plugin{},
		&resetter.Plugin{},
		&informer.Plugin{},
		&server.Plugin{},
		&rpc.Plugin{},
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
	client, err := temporalClient.NewClient(temporalClient.Options{
		HostPort:      "127.0.0.1:7233",
		Namespace:     "default",
		Logger:        newZapAdapter(initLogger()),
		DataConverter: dc,
	})
	require.NoError(t, err)

	return &TestServer{
		client: client,
	}
}

func NewTestServer(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup) *TestServer {
	container, err := endure.NewContainer(initLogger(), endure.RetryOnFail(false), endure.GracefulShutdownTimeout(time.Second*30))
	assert.NoError(t, err)

	cfg := &configImpl.Plugin{
		Timeout: time.Second * 30,
	}
	cfg.Path = "configs/.rr-proto.yaml"
	cfg.Prefix = "rr"

	err = container.RegisterAll(
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
	client, err := temporalClient.NewClient(temporalClient.Options{
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
		client: client,
	}
}

func initConfigProtoWithMetrics() config.Configurer {
	cfg := &configImpl.Plugin{
		Timeout: time.Second * 30,
	}
	cfg.Path = "configs/.rr-metrics.yaml"
	cfg.Prefix = "rr"

	return cfg
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

func (s *TestServer) AssertContainsEvent(t *testing.T, w temporalClient.WorkflowRun, assert func(*history.HistoryEvent) bool) {
	dc := data_converter.NewDataConverter(converter.GetDefaultDataConverter())
	client, err := temporalClient.NewClient(temporalClient.Options{
		HostPort:      "127.0.0.1:7233",
		Namespace:     "default",
		Logger:        newZapAdapter(initLogger()),
		DataConverter: dc,
	})
	require.NoError(t, err)

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

func (s *TestServer) AssertNotContainsEvent(t *testing.T, w temporalClient.WorkflowRun, assert func(*history.HistoryEvent) bool) {
	dc := data_converter.NewDataConverter(converter.GetDefaultDataConverter())
	client, err := temporalClient.NewClient(temporalClient.Options{
		HostPort:           "",
		Namespace:          "default",
		Logger:             nil,
		MetricsHandler:     nil,
		Identity:           "",
		DataConverter:      dc,
		ContextPropagators: nil,
		ConnectionOptions: temporalClient.ConnectionOptions{
			TLS:                          nil,
			Authority:                    "",
			DisableHealthCheck:           false,
			HealthCheckAttemptTimeout:    0,
			HealthCheckTimeout:           0,
			EnableKeepAliveCheck:         false,
			KeepAliveTime:                0,
			KeepAliveTimeout:             0,
			KeepAlivePermitWithoutStream: false,
			MaxPayloadSize:               0,
			DialOptions:                  nil,
		},
		HeadersProvider:   nil,
		TrafficController: nil,
		Interceptors:      nil,
	})
	require.NoError(t, err)

	i := client.GetWorkflowHistory(
		context.Background(),
		w.GetID(),
		w.GetRunID(),
		false,
		enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT,
	)

	for {
		if !i.HasNext() {
			break
		}

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

func (s *TestServer) Client() temporalClient.Client {
	return s.client
}
