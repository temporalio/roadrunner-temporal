package logger

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"
)

// SlogAdapter bridges an slog.Logger to the Temporal SDK log.Logger interface.
type SlogAdapter struct {
	log *slog.Logger
}

// NewSlogAdapter wraps the given slog.Logger into the Temporal SDK log interface.
func NewSlogAdapter(log *slog.Logger) *SlogAdapter {
	return &SlogAdapter{
		log: log,
	}
}

func (l *SlogAdapter) Debug(msg string, keyvals ...any) {
	l.emit(slog.LevelDebug, msg, keyvals)
}

func (l *SlogAdapter) Info(msg string, keyvals ...any) {
	l.emit(slog.LevelInfo, msg, keyvals)
}

func (l *SlogAdapter) Warn(msg string, keyvals ...any) {
	l.emit(slog.LevelWarn, msg, keyvals)
}

func (l *SlogAdapter) Error(msg string, keyvals ...any) {
	l.emit(slog.LevelError, msg, keyvals)
}

// emit builds the record manually so the source location points at the
// Temporal SDK call site instead of this adapter.
func (l *SlogAdapter) emit(level slog.Level, msg string, keyvals []any) {
	ctx := context.Background()
	if !l.log.Enabled(ctx, level) {
		return
	}

	var pcs [1]uintptr
	// skip runtime.Callers, emit and the exported wrapper method
	runtime.Callers(3, pcs[:])

	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(normalize(keyvals)...)

	_ = l.log.Handler().Handle(ctx, r)
}

// normalize keeps key/value pairs aligned: slog consumes a single element for a
// non-string key, which would shift every pair after it.
func normalize(keyvals []any) []any {
	if len(keyvals)%2 != 0 {
		return []any{"error", fmt.Errorf("odd number of keyvals pairs: %v", keyvals)}
	}

	for i := 0; i < len(keyvals); i += 2 {
		if _, ok := keyvals[i].(string); !ok {
			keyvals[i] = fmt.Sprintf("%v", keyvals[i])
		}
	}

	return keyvals
}
