package rrtemporal

import (
	"io"
	"strconv"
	"time"

	"github.com/cactus/go-statsd-client/v5/statsd"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/pool/fsm"
	"github.com/roadrunner-server/pool/state/process"
	"github.com/uber-go/tally/v4"
	"github.com/uber-go/tally/v4/prometheus"
	tclient "go.temporal.io/sdk/client"
	ttally "go.temporal.io/sdk/contrib/tally" // temporal tally hanlders
	statsdreporter "go.temporal.io/server/common/metrics/tally/statsd"
	"go.uber.org/zap"
)

const (
	namespace = "rr_temporal"
)

func (p *Plugin) MetricsCollector() []prom.Collector {
	// p - implements Exporter interface (workers)
	// other - request duration and count
	return []prom.Collector{p.statsExporter}
}

// Informer used to get workers from a particular plugin or set of plugins
type Informer interface {
	Workers() []*process.State
}

func newStatsExporter(stats Informer) *StatsExporter {
	return &StatsExporter{
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
	// This will rename metrics at the tally emission level, so the metrics name we use is
	//  maybe different from what gets emitted. In the current implementation
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

// init RR metrics
func initMetrics(cfg *Config, log *zap.Logger) (tclient.MetricsHandler, io.Closer, error) {
	switch cfg.Metrics.Driver {
	case driverPrometheus:
		ms, cl, err := newPrometheusScope(prometheus.Configuration{
			ListenAddress: cfg.Metrics.Prometheus.Address,
			TimerType:     cfg.Metrics.Prometheus.Type,
		}, cfg.Metrics.Prometheus.Prefix, log)
		if err != nil {
			return nil, nil, err
		}

		return ttally.NewMetricsHandler(ms), cl, nil
	case driverStatsd:
		ms, cl, err := newStatsdScope(cfg.Metrics.Statsd)
		if err != nil {
			return nil, nil, err
		}
		return ttally.NewMetricsHandler(ms), cl, nil
	default:
		return nil, nil, errors.Errorf("unknown driver provided: %s", cfg.Metrics.Driver)
	}
}

type StatsExporter struct {
	TotalMemoryDesc  *prom.Desc
	StateDesc        *prom.Desc
	WorkerMemoryDesc *prom.Desc
	TotalWorkersDesc *prom.Desc

	WorkersReady   *prom.Desc
	WorkersWorking *prom.Desc
	WorkersInvalid *prom.Desc

	Workers Informer
}

func (s *StatsExporter) Describe(d chan<- *prom.Desc) {
	// send description
	d <- s.TotalWorkersDesc
	d <- s.TotalMemoryDesc
	d <- s.StateDesc
	d <- s.WorkerMemoryDesc

	d <- s.WorkersReady
	d <- s.WorkersWorking
	d <- s.WorkersInvalid
}

func (s *StatsExporter) Collect(ch chan<- prom.Metric) {
	// get the copy of the processes
	workerStates := s.Workers.Workers()

	// cumulative RSS memory in bytes
	var cum float64

	var ready float64
	var working float64
	var invalid float64

	// collect the memory
	for i := 0; i < len(workerStates); i++ {
		cum += float64(workerStates[i].MemoryUsage)

		ch <- prom.MustNewConstMetric(s.StateDesc, prom.GaugeValue, 0, workerStates[i].StatusStr, strconv.Itoa(int(workerStates[i].Pid)))
		ch <- prom.MustNewConstMetric(s.WorkerMemoryDesc, prom.GaugeValue, float64(workerStates[i].MemoryUsage), strconv.Itoa(int(workerStates[i].Pid)))

		// sync with sdk/worker/state.go
		switch workerStates[i].Status {
		case fsm.StateReady:
			ready++
		case fsm.StateWorking:
			working++
		default:
			invalid++
		}
	}

	ch <- prom.MustNewConstMetric(s.WorkersReady, prom.GaugeValue, ready)
	ch <- prom.MustNewConstMetric(s.WorkersWorking, prom.GaugeValue, working)
	ch <- prom.MustNewConstMetric(s.WorkersInvalid, prom.GaugeValue, invalid)

	// send the values to the prometheus
	ch <- prom.MustNewConstMetric(s.TotalWorkersDesc, prom.GaugeValue, float64(len(workerStates)))
	ch <- prom.MustNewConstMetric(s.TotalMemoryDesc, prom.GaugeValue, cum)
}
