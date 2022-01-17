package roadrunner_temporal //nolint:revive,stylecheck

import "github.com/roadrunner-server/sdk/v2/pool"

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
}

func (c *Config) InitDefault() {
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
}
