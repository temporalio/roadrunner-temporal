package factory

import (
	"github.com/temporalio/roadrunner-temporal/config"
	"os/exec"
)

// AppConfig config combines factory, pool and cmd configurations.
type AppConfig struct {
	Command string
	User    string
	Group   string
	Env     Env
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
