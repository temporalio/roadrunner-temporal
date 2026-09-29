package mocklogger

import (
	"context"
	"log/slog"
	"strings"
	"sync"
)

// LoggedEntry is a representation of a log record captured by the observer.
type LoggedEntry struct {
	Level   slog.Level
	Message string
	Attrs   map[string]any
}

// ObservedLogs is a concurrency-safe, ordered collection of observed logs.
type ObservedLogs struct {
	mu   sync.RWMutex
	logs []LoggedEntry
}

// Len returns the number of items in the collection.
func (o *ObservedLogs) Len() int {
	o.mu.RLock()
	n := len(o.logs)
	o.mu.RUnlock()
	return n
}

// All returns a copy of all the observed logs.
func (o *ObservedLogs) All() []LoggedEntry {
	o.mu.RLock()
	ret := make([]LoggedEntry, len(o.logs))
	copy(ret, o.logs)
	o.mu.RUnlock()
	return ret
}

// FilterMessageSnippet returns the entries whose message contains the snippet.
func (o *ObservedLogs) FilterMessageSnippet(snippet string) *ObservedLogs {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var filtered []LoggedEntry
	for _, entry := range o.logs {
		if strings.Contains(entry.Message, snippet) {
			filtered = append(filtered, entry)
		}
	}

	return &ObservedLogs{logs: filtered}
}

func (o *ObservedLogs) add(entry LoggedEntry) {
	o.mu.Lock()
	o.logs = append(o.logs, entry)
	o.mu.Unlock()
}

// observerHandler is an slog.Handler that captures the level, the message, and
// the attributes of every record. Attributes added with WithAttrs are not recorded.
type observerHandler struct {
	level slog.Level
	logs  *ObservedLogs
}

// NewObserverHandler creates a new slog.Handler that buffers logs in memory.
func NewObserverHandler(level slog.Level) (slog.Handler, *ObservedLogs) {
	ol := &ObservedLogs{}
	return &observerHandler{
		level: level,
		logs:  ol,
	}, ol
}

func (h *observerHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *observerHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := make(map[string]any, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	h.logs.add(LoggedEntry{
		Level:   r.Level,
		Message: r.Message,
		Attrs:   attrs,
	})
	return nil
}

func (h *observerHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *observerHandler) WithGroup(_ string) slog.Handler {
	return h
}
