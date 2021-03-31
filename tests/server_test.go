package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	endure "github.com/spiral/endure/pkg/container"
	"github.com/spiral/roadrunner/v2/plugins/config"
	"github.com/spiral/roadrunner/v2/plugins/informer"
	"github.com/spiral/roadrunner/v2/plugins/logger"
	"github.com/spiral/roadrunner/v2/plugins/resetter"
	"github.com/spiral/roadrunner/v2/plugins/rpc"
	"github.com/spiral/roadrunner/v2/plugins/server"
	"github.com/stretchr/testify/assert"
	"github.com/temporalio/roadrunner-temporal/activity"
	rrClient "github.com/temporalio/roadrunner-temporal/client"
	"github.com/temporalio/roadrunner-temporal/workflow"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	temporalClient "go.temporal.io/sdk/client"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type TestServer struct {
	temporal   rrClient.Temporal
	activities *activity.Plugin
	workflows  *workflow.Plugin
}

func NewTestServer(t *testing.T, stopCh chan struct{}, wg *sync.WaitGroup, proto bool) *TestServer {
	container, err := endure.NewContainer(initLogger(), endure.RetryOnFail(false))
	assert.NoError(t, err)

	tc := &rrClient.Plugin{}
	a := &activity.Plugin{}
	w := &workflow.Plugin{}

	cfg := initConfigJSON()
	if proto {
		cfg = initConfigProto()
	}

	err = container.RegisterAll(
		tc,
		a,
		w,
		cfg,
		&logger.ZapLogger{},
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

	return &TestServer{temporal: tc, activities: a, workflows: w}
}

func (s *TestServer) Client() temporalClient.Client {
	return s.temporal.GetClient()
}

func initConfigJSON() config.Configurer {
	cfg := &config.Viper{
		CommonConfig: &config.General{GracefulTimeout: time.Second * 0},
	}
	cfg.Path = ".rr.yaml"
	cfg.Prefix = "rr"

	return cfg
}

func initConfigProto() config.Configurer {
	cfg := &config.Viper{
		CommonConfig: &config.General{GracefulTimeout: time.Second * 0},
	}
	cfg.Path = ".rr-proto.yaml"
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
	i := s.Client().GetWorkflowHistory(
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
	i := s.Client().GetWorkflowHistory(
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
