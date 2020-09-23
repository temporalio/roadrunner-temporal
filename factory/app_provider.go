package factory

import (
	"os/exec"
)

type Env map[string]string

type AppProvider interface {
	// CmdFactory create new command factory with given env variables.
	CmdFactory(env Env) (func() *exec.Cmd, error)
}
