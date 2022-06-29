package roadrunner_temporal //nolint:revive,stylecheck

import (
	"io"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/roadrunner-server/api/v2/plugins/informer"
	"github.com/roadrunner-server/sdk/v2/metrics"
	"github.com/uber-go/tally/v4"
	"github.com/uber-go/tally/v4/prometheus"
	"go.uber.org/zap"
)

// tally sanitizer options that satisfy Prometheus restrictions.
// This will rename metrics at the tally emission level, so metrics name we
// use maybe different from what gets emitted. In the current implementation
// it will replace - and . with _
var (
	safeCharacters = []rune{'_'}

	sanitizeOptions = tally.SanitizeOptions{
		NameCharacters: tally.ValidCharacters{
			Ranges:     tally.AlphanumericRange,
			Characters: safeCharacters,
		},
		KeyCharacters: tally.ValidCharacters{
			Ranges:     tally.AlphanumericRange,
			Characters: safeCharacters,
		},
		ValueCharacters: tally.ValidCharacters{
			Ranges:     tally.AlphanumericRange,
			Characters: safeCharacters,
		},
		ReplacementCharacter: tally.DefaultReplacementCharacter,
	}
)

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
	scopeOpts := tally.ScopeOptions{
		CachedReporter:  reporter,
		Separator:       prometheus.DefaultSeparator,
		SanitizeOptions: &sanitizeOptions,
		Prefix:          prefix,
	}
	scope, closer := tally.NewRootScope(scopeOpts, time.Second)

	return scope, closer, nil
}

func (p *Plugin) MetricsCollector() []prom.Collector {
	// p - implements Exporter interface (workers)
	// other - request duration and count
	return []prom.Collector{p.statsExporter}
}

const (
	namespace = "rr_temporal"
)

func newStatsExporter(stats informer.Informer) *metrics.StatsExporter {
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
