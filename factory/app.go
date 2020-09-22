package factory

import (
	"github.com/temporalio/roadrunner-temporal/config"
	"os/exec"
	"time"
)

// AppConfig config combines factory, pool and cmd configurations.
type AppConfig struct {
	Command string

	User  string
	Group string

	Env Env

	// Relay defines connection method and factory to be used to connect to workers:
	// "pipes", "tcp://:6001", "unix://rr.sock"
	// This config section must not change on re-configuration.
	Relay string

	// RelayTimeout defines for how long socket factory will be waiting for worker connection. This config section
	// must not change on re-configuration.
	RelayTimeout time.Duration

	// todo: connect timeouts to the app?
	// todo: destroy timeouts to the app?
}

type App struct {
	cfg *AppConfig
}

func (app *App) Init(cfg config.Provider) error {
	app.cfg = &AppConfig{}

	return cfg.UnmarshalKey("app", app.cfg)
}

func (app *App) CmdFactory(env Env) (func() *exec.Cmd, error) {

}
