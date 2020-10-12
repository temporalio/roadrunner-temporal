package factory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/temporalio/roadrunner-temporal/config"
	"github.com/temporalio/roadrunner-temporal/roadrunner"
	"github.com/temporalio/roadrunner-temporal/roadrunner/util"
)

// AppConfig config combines factory, pool and cmd configurations.
type AppConfig struct {
	Command string
	User    string
	Group   string
	Env     Env

	// Relay defines connection method and factory to be used to connect to workers:
	// "pipes", "tcp://:6001", "unix://rr.sock"
	// This config section must not change on re-configuration.
	Relay string

	// RelayTimeout defines for how long socket factory will be waiting for worker connection. This config section
	// must not change on re-configuration.
	RelayTimeout time.Duration
}

type App struct {
	cfg            *AppConfig
	configProvider config.Provider
	factory        roadrunner.Factory
}

func (app *App) Init(provider config.Provider) error {
	app.cfg = &AppConfig{}
	app.configProvider = provider

	return nil
}

func (app *App) Configure() error {
	err := app.configProvider.UnmarshalKey("app", app.cfg)
	if err != nil {
		return err
	}
	return nil
}

func (app *App) Close() error {
	return nil
}

func (app *App) NewCmd(env Env) (func() *exec.Cmd, error) {
	var cmdArgs []string
	// create command according to the config
	cmdArgs = append(cmdArgs, strings.Split(app.cfg.Command, " ")...)

	return func() *exec.Cmd {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		util.IsolateProcess(cmd)

		// if user is not empty, and OS is linux or macos
		// execute php worker from that particular user
		if app.cfg.User != "" {
			err := util.ExecuteFromUser(cmd, app.cfg.User)
			if err != nil {
				return nil
			}
		}

		cmd.Env = app.setEnv(env)

		return cmd
	}, nil
}

func (app *App) NewFactory(env Env) (roadrunner.Factory, error) {
	// TODO unix missed
	if app.cfg.Relay == "" {
		return nil, errors.New("relay should be set")
	}

	switch app.cfg.Relay {
	case "sockets":
		return roadrunner.NewSocketServer(nil, time.Second*100), nil
	case "pipes":
		return roadrunner.NewPipeFactory(), nil
	}
	return roadrunner.NewPipeFactory(), nil
}

func (app *App) Serve() chan error {
	errCh := make(chan error)
	return errCh
}

func (app *App) Stop() error {
	err := app.factory.Close(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func (app *App) setEnv(e Env) []string {
	env := append(os.Environ(), fmt.Sprintf("RR_RELAY=%s", app.cfg.Relay))
	for k, v := range e {
		env = append(env, fmt.Sprintf("%s=%s", strings.ToUpper(k), v))
	}

	return env
}
