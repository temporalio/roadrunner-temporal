package roadrunner_temporal //nolint:revive,stylecheck

import (
	"crypto/tls"
	"os"
	"time"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v3/pool"
)

const (
	MetricsTypeSummary string = "summary"

	// metrics types

	driverPrometheus string = "prometheus"
	driverStatsd     string = "statsd"
)

type ClientAuthType string

const (
	NoClientCert               ClientAuthType = "no_client_cert"
	RequestClientCert          ClientAuthType = "request_client_cert"
	RequireAnyClientCert       ClientAuthType = "require_any_client_cert"
	VerifyClientCertIfGiven    ClientAuthType = "verify_client_cert_if_given"
	RequireAndVerifyClientCert ClientAuthType = "require_and_verify_client_cert"
)

// ref:https://github.dev/temporalio/temporal/common/metrics/config.go:79
type Statsd struct {
	// The host and port of the statsd server
	HostPort string `mapstructure:"host_port" validate:"nonzero"`
	// The prefix to use in reporting to statsd
	Prefix string `mapstructure:"prefix" validate:"nonzero"`
	// FlushInterval is the maximum interval for sending packets.
	// If it is not specified, it defaults to 1 second.
	FlushInterval time.Duration `mapstructure:"flush_interval"`
	// FlushBytes specifies the maximum udp packet size you wish to send.
	// If FlushBytes is unspecified, it defaults  to 1432 bytes, which is
	// considered safe for local traffic.
	FlushBytes int `mapstructure:"flush_bytes"`
	// Tags to pass to the Tally scope options
	Tags map[string]string `mapstructure:"tags"`
	// TagPrefix ...
	TagPrefix string `mapstructure:"tag_prefix"`
	// TagSeparator allows tags to be appended with a separator. If not specified tag keys and values
	// are embedded to the stat name directly.
	TagSeparator string `mapstructure:"tag_separator"`
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
	Key        string         `mapstructure:"key"`
	Cert       string         `mapstructure:"cert"`
	RootCA     string         `mapstructure:"root_ca"`
	AuthType   ClientAuthType `mapstructure:"client_auth_type"`
	ServerName string         `mapstructure:"server_name"`
	// auth type
	auth tls.ClientAuthType
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

		// RootCA is optional, but if provided - check it
		if c.TLS.RootCA != "" {
			if _, err := os.Stat(c.TLS.RootCA); err != nil {
				if os.IsNotExist(err) {
					return errors.E(op, errors.Errorf("root ca path provided, but key file '%s' does not exists", c.TLS.RootCA))
				}
				return errors.E(op, err)
			}

			// auth type used only for the CA
			switch c.TLS.AuthType {
			case NoClientCert:
				c.TLS.auth = tls.NoClientCert
			case RequestClientCert:
				c.TLS.auth = tls.RequestClientCert
			case RequireAnyClientCert:
				c.TLS.auth = tls.RequireAnyClientCert
			case VerifyClientCertIfGiven:
				c.TLS.auth = tls.VerifyClientCertIfGiven
			case RequireAndVerifyClientCert:
				c.TLS.auth = tls.RequireAndVerifyClientCert
			default:
				c.TLS.auth = tls.NoClientCert
			}
		}
	}

	return nil
}
