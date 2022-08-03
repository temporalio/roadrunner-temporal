package roadrunner_temporal //nolint:revive,stylecheck

import (
	"os"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v2/pool"
)

const (
	MetricsTypeSummary string = "summary"
)

type Metrics struct {
	Address string `mapstructure:"address"`
	Type    string `mapstructure:"type"`
	Prefix  string `mapstructure:"prefix"`
}

// Config of the temporal client and dependent services.
type Config struct {
	Address    string       `mapstructure:"address"`
	Namespace  string       `mapstructure:"namespace"`
	Metrics    *Metrics     `mapstructure:"metrics"`
	Activities *pool.Config `mapstructure:"activities"`
	CacheSize  int          `mapstructure:"cache_size"`
	TLS        *TLS         `mapstructure:"tls, omitempty"`
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
		if c.Metrics.Type == "" {
			c.Metrics.Type = MetricsTypeSummary
		}

		if c.Metrics.Address == "" {
			c.Metrics.Address = "127.0.0.1:9091"
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
