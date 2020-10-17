package main

import (
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"log"

	"github.com/spiral/endure"
	"github.com/spiral/roadrunner/v2/plugins/factory"
	"github.com/temporalio/roadrunner-temporal/cmd/cli"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// LOG LEVEL SHOULD BE SET BY CLI, todo: move from main.go
	cfg := zap.Config{
		Level:    zap.NewAtomicLevelAt(zap.DebugLevel),
		Encoding: "console",
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:    "message",
			LevelKey:      "level",
			TimeKey:       "time",
			CallerKey:     "caller",
			StacktraceKey: "stack",
			EncodeLevel:   zapcore.CapitalLevelEncoder,
			EncodeTime:    zapcore.ISO8601TimeEncoder,
			EncodeCaller:  zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := cfg.Build(zap.AddCaller())
	if err != nil {
		log.Fatal("failed to initialize logger")
		return
	}

	// todo: verbose and debug flag
	cli.Container, err = endure.NewContainer(endure.DebugLevel, endure.RetryOnFail(false))
	if err != nil {
		logger.Fatal("failed to instantiate endure container", zap.Error(err))
		return
	}

	err = cli.Container.Register(&factory.App{})
	if err != nil {
		logger.Fatal("failed to factory App", zap.Error(err))
		return
	}

	err = cli.Container.Register(&factory.WFactory{})
	if err != nil {
		logger.Fatal("failed to register WFactory", zap.Error(err))
		return
	}

	err = cli.Container.Register(&temporal.Provider{})
	if err != nil {
		logger.Fatal("failed to register temporal plugin", zap.Error(err))
		return
	}

	err = cli.Container.Register(&temporal.ActivityPool{})
	if err != nil {
		logger.Fatal("failed to register temporal activity pool", zap.Error(err))
		return
	}

	// exec
	cli.Execute()
}
