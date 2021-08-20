package client

import "github.com/spiral/roadrunner/v2/pkg/pool"

const (
	MetricsTypeSummary string = "summary"
)

type Metrics struct {
	Address string `mapstructure:"address"`
	Type    string `mapstructure:"type"`
}

// Config of the temporal client and depended services.
type Config struct {
	Address    string
	Namespace  string
	Metrics    *Metrics
	Activities *pool.Config
	Codec      string
	DebugLevel int `mapstructure:"debug_level"`
	CacheSize  int `mapstructure:"cache_size"`
}

func (c *Config) InitDefault() {
	if c.Activities != nil {
		c.Activities.InitDefaults()
	}

	if c.Codec == "" {
		c.Codec = "proto"
	}

	if c.CacheSize == 0 {
		c.CacheSize = 10000
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
