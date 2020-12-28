package tests

import (
	"context"
	"github.com/spiral/endure"
	"github.com/spiral/roadrunner/v2/plugins/config"
	"github.com/spiral/roadrunner/v2/plugins/informer"
	"github.com/spiral/roadrunner/v2/plugins/logger"
	"github.com/spiral/roadrunner/v2/plugins/resetter"
	"github.com/spiral/roadrunner/v2/plugins/rpc"
	"github.com/spiral/roadrunner/v2/plugins/server"
	"github.com/temporalio/roadrunner-temporal/plugins/activity"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"github.com/temporalio/roadrunner-temporal/plugins/workflow"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"testing"
)

type TestServer struct {
	container  endure.Container
	temporal   temporal.Temporal
	activities *activity.Plugin
	workflows  *workflow.Plugin
}

type ConfigOption struct {
	Name  string
	Value interface{}
}

func NewOption(name string, value interface{}) ConfigOption {
	return ConfigOption{Name: name, Value: value}
}

func NewTestServer(opt ...ConfigOption) *TestServer {
	e, err := endure.NewContainer(initLogger(), endure.RetryOnFail(false))
	if err != nil {
		panic(err)
	}

	t := &temporal.Plugin{}
	a := &activity.Plugin{}
	w := &workflow.Plugin{}

	if err := e.Register(initConfig(opt...)); err != nil {
		panic(err)
	}

	if err := e.Register(&logger.ZapLogger{}); err != nil {
		panic(err)
	}
	if err := e.Register(&resetter.Plugin{}); err != nil {
		panic(err)
	}
	if err := e.Register(&informer.Plugin{}); err != nil {
		panic(err)
	}
	if err := e.Register(&server.Plugin{}); err != nil {
		panic(err)
	}
	if err := e.Register(&rpc.Plugin{}); err != nil {
		panic(err)
	}

	if err := e.Register(t); err != nil {
		panic(err)
	}
	if err := e.Register(a); err != nil {
		panic(err)
	}
	if err := e.Register(w); err != nil {
		panic(err)
	}

	if err := e.Init(); err != nil {
		panic(err)
	}

	errCh, err := e.Serve()
	if err != nil {
		panic(err)
	}

	go func() {
		err := <-errCh
		er := e.Stop()
		if er != nil {
			panic(err)
		}
	}()

	return &TestServer{container: e, temporal: t, activities: a, workflows: w}
}

func (t *TestServer) Client() client.Client {
	c, err := t.temporal.GetClient()
	if err != nil {
		panic(err)
	}

	return c
}

func (t *TestServer) MustClose() {
	err := t.container.Stop()
	if err != nil {
		panic(err)
	}
}

func initConfig(opt ...ConfigOption) config.Configurer {
	// todo: mock config
	cfg := &config.Viper{}
	cfg.Path = ".rr.yaml"
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

func (s *TestServer) AssertContainsEvent(t *testing.T, w client.WorkflowRun, assert func(*history.HistoryEvent) bool) {
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

func (s *TestServer) AssertNotContainsEvent(t *testing.T, w client.WorkflowRun, assert func(*history.HistoryEvent) bool) {
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
