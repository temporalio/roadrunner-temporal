package factory

import (
	"github.com/temporalio/roadrunner-temporal/roadrunner"
	"os/exec"
)

type Env map[string]string

type AppProvider interface {
	// CmdFactory create new command factory with given env variables.
	CmdFactory(env Env) (func() *exec.Cmd, error)

	//// NewFactory inits new factory for workers.
	//SpawnWorker(env Env) (*roadrunner.Worker, error)
}
