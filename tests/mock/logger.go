package mocklogger

import (
	"github.com/roadrunner-server/endure/v2/dep"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ZapLoggerMock struct {
	l *zap.Logger
}

type Logger interface {
	NamedLogger(string) *zap.Logger
}

func ZapTestLogger(enab zapcore.LevelEnabler) (*ZapLoggerMock, *ObservedLogs) {
	core, logs := New(enab)
	obsLog := zap.New(core, zap.Development())

	return &ZapLoggerMock{
		l: obsLog,
	}, logs
}

func (z *ZapLoggerMock) Init() error {
	return nil
}

func (z *ZapLoggerMock) Serve() chan error {
	return make(chan error, 1)
}

func (z *ZapLoggerMock) Stop() error {
	return z.l.Sync()
}

func (z *ZapLoggerMock) Provides() []*dep.Out {
	return []*dep.Out{
		dep.Bind((*Logger)(nil), z.ProvideLogger),
	}
}

func (z *ZapLoggerMock) Weight() uint {
	return 100
}

func (z *ZapLoggerMock) ProvideLogger() *Log {
	return NewLogger(z.l)
}

type Log struct {
	base *zap.Logger
}

func NewLogger(log *zap.Logger) *Log {
	return &Log{
		base: log,
	}
}

func (l *Log) NamedLogger(string) *zap.Logger {
	return l.base
}
