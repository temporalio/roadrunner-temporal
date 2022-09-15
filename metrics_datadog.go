// Based on https://github.com/temporalio/temporal/pull/998#issuecomment-857884983
package roadrunner_temporal

import (
	"fmt"
	"go.uber.org/zap"
	"sort"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/uber-go/tally/v4"
)

type dogstatsdReporter struct {
	dogstatsd *statsd.Client
	log       *zap.Logger
}

func NewDataDogMetricsReporter(statsdConfig *Statsd, log *zap.Logger) tally.StatsReporter {
	client, err := statsd.New(
		statsdConfig.HostPort,
		statsd.WithBufferFlushInterval(statsdConfig.FlushInterval),
		statsd.WithMaxBytesPerPayload(statsdConfig.FlushBytes),
	)
	if err != nil {
		log.Fatal("error creating dogstatsd client", zap.Error(err))
	}
	return &dogstatsdReporter{
		dogstatsd: client,
		log:       log,
	}
}

func (r dogstatsdReporter) Capabilities() tally.Capabilities {
	return r
}

func (r dogstatsdReporter) Reporting() bool {
	return true
}

func (r dogstatsdReporter) Tagging() bool {
	return true
}

func (r dogstatsdReporter) Flush() {
	if err := r.dogstatsd.Flush(); err != nil {
		r.log.Error("error while flushing", zap.Error(err))
	}
}

func (r dogstatsdReporter) ReportCounter(name string, tags map[string]string, value int64) {
	if err := r.dogstatsd.Count(name, value, r.marshalTags(tags), 1); err != nil {
		r.log.Error("failed reporting counter", zap.Error(err))
	}
}

func (r dogstatsdReporter) ReportGauge(name string, tags map[string]string, value float64) {
	if err := r.dogstatsd.Gauge(name, value, r.marshalTags(tags), 1); err != nil {
		r.log.Error("failed reporting gauge", zap.Error(err))
	}
}

func (r dogstatsdReporter) ReportTimer(name string, tags map[string]string, interval time.Duration) {
	if err := r.dogstatsd.Timing(name, interval, r.marshalTags(tags), 1); err != nil {
		r.log.Error("failed reporting timer", zap.Error(err))
	}
}

func (r dogstatsdReporter) ReportHistogramValueSamples(name string, tags map[string]string, buckets tally.Buckets, bucketLowerBound, bucketUpperBound float64, samples int64) {
	r.log.Warn("unexpected call to ReportHistogramValueSamples")
}

func (r dogstatsdReporter) ReportHistogramDurationSamples(name string, tags map[string]string, buckets tally.Buckets, bucketLowerBound, bucketUpperBound time.Duration, samples int64) {
	r.log.Warn("unexpected call to ReportHistogramDurationSamples")
}

func (r dogstatsdReporter) marshalTags(tags map[string]string) []string {
	var keys []string
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var dogTags []string
	for _, tk := range keys {
		dogTags = append(dogTags, fmt.Sprintf("%s:%s", tk, tags[tk]))
	}

	return dogTags
}
