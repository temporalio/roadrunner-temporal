package roadrunner_temporal //nolint:revive,stylecheck

import (
	"os"
	"time"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v2/pool"
)

const (
	MetricsTypeSummary string = "summary"

	// metrics types

	driverPrometheus string = "prometheus"
	driverStatsd     string = "statsd"
)

// ref:https://github.dev/temporalio/temporal/common/metrics/config.go:79
type Statsd struct {
	// The host and port of the statsd server
	HostPort string `yaml:"host_port" validate:"nonzero"`
	// The prefix to use in reporting to statsd
	Prefix string `yaml:"prefix" validate:"nonzero"`
	// FlushInterval is the maximum interval for sending packets.
	// If it is not specified, it defaults to 1 second.
	FlushInterval time.Duration `yaml:"flush_interval"`
	// FlushBytes specifies the maximum udp packet size you wish to send.
	// If FlushBytes is unspecified, it defaults  to 1432 bytes, which is
	// considered safe for local traffic.
	FlushBytes int `yaml:"flush_bytes"`
	// Reporter allows additional configuration of the stats reporter, e.g. with custom tagging options.
	Reporter *StatsdReporterConfig `yaml:"reporter"`
}

type StatsdReporterConfig struct {
	// TagSeparator allows tags to be appended with a separator. If not specified tag keys and values
	// are embedded to the stat name directly.
	TagSeparator string `yaml:"tag_separator"`
}

type Prometheus struct {
	Address string `mapstructure:"address"`
	Type    string `mapstructure:"type"`
	Prefix  string `mapstructure:"prefix"`
}

type Metrics struct {
	Driver     string      `mapstructure:"driver"`
	Prometheus *Prometheus `mapstructure:"prometheus"`
	Statsd     *Statsd     `mapstructure:"statsd"`
}

// Config of the temporal client and dependent services.
type Config struct {
	Metrics    *Metrics     `mapstructure:"metrics"`
	Activities *pool.Config `mapstructure:"activities"`
	TLS        *TLS         `mapstructure:"tls, omitempty"`

	Address   string `mapstructure:"address"`
	Namespace string `mapstructure:"namespace"`
	CacheSize int    `mapstructure:"cache_size"`
}

type TLS struct {
	Key  string `mapstructure:"key"`
	Cert string `mapstructure:"cert"`
}

func (c *Config) InitDefault() error {
	const op = errors.Op("init_defaults_temporal")

	if c.Activities != nil {
		c.Activities.InitDefaults()
	}

	if c.CacheSize == 0 {
		c.CacheSize = 10000
	}

	if c.Namespace == "" {
		c.Namespace = "default"
	}

	if c.Metrics != nil {
		if c.Metrics.Driver == "" {
			c.Metrics.Driver = driverPrometheus
		}

		switch c.Metrics.Driver {
		case driverPrometheus:
			if c.Metrics.Prometheus == nil {
				c.Metrics.Prometheus = &Prometheus{}
			}

			if c.Metrics.Prometheus.Type == "" {
				c.Metrics.Prometheus.Type = MetricsTypeSummary
			}

			if c.Metrics.Prometheus.Address == "" {
				c.Metrics.Prometheus.Address = "127.0.0.1:9091"
			}

		case driverStatsd:
			if c.Metrics.Statsd != nil {
				if c.Metrics.Statsd.HostPort == "" {
					c.Metrics.Statsd.HostPort = "127.0.0.1:8125"
				}

				// init with empty
				if c.Metrics.Statsd.Reporter == nil {
					c.Metrics.Statsd.Reporter = &StatsdReporterConfig{TagSeparator: ""}
				}
			}
		}
	}

	if c.TLS != nil {
		if _, err := os.Stat(c.TLS.Key); err != nil {
			if os.IsNotExist(err) {
				return errors.E(op, errors.Errorf("key file '%s' does not exists", c.TLS.Key))
			}

			return errors.E(op, err)
		}

		if _, err := os.Stat(c.TLS.Cert); err != nil {
			if os.IsNotExist(err) {
				return errors.E(op, errors.Errorf("cert file '%s' does not exists", c.TLS.Cert))
			}

			return errors.E(op, err)
		}
	}

	return nil
}
