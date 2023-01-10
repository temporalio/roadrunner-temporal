package roadrunner_temporal //nolint:revive,stylecheck

import (
	"io"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/roadrunner-server/sdk/v4/metrics"
	"github.com/roadrunner-server/sdk/v4/state/process"
	"github.com/uber-go/tally/v4"
	"github.com/uber-go/tally/v4/prometheus"
	statsdreporter "go.temporal.io/server/common/metrics/tally/statsd"
	"go.uber.org/zap"
)

const (
	namespace = "rr_temporal"
)

// Informer used to get workers from particular plugin or set of plugins
type Informer interface {
	Workers() []*process.State
}

func newStatsExporter(stats Informer) *metrics.StatsExporter {
	return &metrics.StatsExporter{
		TotalMemoryDesc:  prom.NewDesc(prom.BuildFQName(namespace, "", "workers_memory_bytes"), "Memory usage by workers", nil, nil),
		StateDesc:        prom.NewDesc(prom.BuildFQName(namespace, "", "worker_state"), "Worker current state", []string{"state", "pid"}, nil),
		WorkerMemoryDesc: prom.NewDesc(prom.BuildFQName(namespace, "", "worker_memory_bytes"), "Worker current memory usage", []string{"pid"}, nil),
		TotalWorkersDesc: prom.NewDesc(prom.BuildFQName(namespace, "", "total_workers"), "Total number of workers used by the plugin", nil, nil),
		WorkersReady:     prom.NewDesc(prom.BuildFQName(namespace, "", "workers_ready"), "Workers currently in ready state", nil, nil),
		WorkersWorking:   prom.NewDesc(prom.BuildFQName(namespace, "", "workers_working"), "Workers currently in working state", nil, nil),
		WorkersInvalid:   prom.NewDesc(prom.BuildFQName(namespace, "", "workers_invalid"), "Workers currently in invalid,killing,destroyed,errored,inactive states", nil, nil),
		Workers:          stats,
	}
}

func newPrometheusScope(c prometheus.Configuration, prefix string, log *zap.Logger) (tally.Scope, io.Closer, error) {
	reporter, err := c.NewReporter(
		prometheus.ConfigurationOptions{
			Registry: prom.NewRegistry(),
			OnError: func(err error) {
				log.Error("prometheus registry", zap.Error(err))
			},
		},
	)
	if err != nil {
		return nil, nil, err
	}

	// tally sanitizer options that satisfy Prometheus restrictions.
	// This will rename metrics at the tally emission level, so metrics name we
	// use maybe different from what gets emitted. In the current implementation
	// it will replace - and . with _
	sanitizeOptions := tally.SanitizeOptions{
		NameCharacters: tally.ValidCharacters{
			Ranges:     tally.AlphanumericRange,
			Characters: []rune{'_'},
		},
		KeyCharacters: tally.ValidCharacters{
			Ranges:     tally.AlphanumericRange,
			Characters: []rune{'_'},
		},
		ValueCharacters: tally.ValidCharacters{
			Ranges:     tally.AlphanumericRange,
			Characters: []rune{'_'},
		},
		ReplacementCharacter: tally.DefaultReplacementCharacter,
	}

	scopeOpts := tally.ScopeOptions{
		CachedReporter:  reporter,
		Separator:       prometheus.DefaultSeparator,
		SanitizeOptions: &sanitizeOptions,
		Prefix:          prefix,
	}
	scope, closer := tally.NewRootScope(scopeOpts, time.Second)

	return scope, closer, nil
}

// ref: https://github.dev/temporalio/temporal/common/metrics/config.go:391
func newStatsdScope(statsdConfig *Statsd) (tally.Scope, io.Closer, error) {
	st, err := statsd.NewClientWithConfig(&statsd.ClientConfig{
		Address:       statsdConfig.HostPort,
		Prefix:        statsdConfig.Prefix,
		UseBuffered:   true,
		FlushInterval: statsdConfig.FlushInterval,
		FlushBytes:    statsdConfig.FlushBytes,
	})

	if err != nil {
		return nil, nil, err
	}

	// NOTE: according to (https://github.com/uber-go/tally) Tally's statsd implementation doesn't support tagging.
	// Therefore, we implement Tally interface to have a statsd reporter that can support tagging
	opts := statsdreporter.Options{
		TagSeparator: statsdConfig.TagSeparator,
	}

	reporter := statsdreporter.NewReporter(st, opts)
	scopeOpts := tally.ScopeOptions{
		Tags:     statsdConfig.Tags,
		Prefix:   statsdConfig.Prefix,
		Reporter: reporter,
	}

	scope, closer := tally.NewRootScope(scopeOpts, time.Second)

	return scope, closer, nil
}
