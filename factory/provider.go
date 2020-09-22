package factory

import "os/exec"

type Env map[string]string

type Provider interface {
	// CmdFactory create new command factory with given env variables.
	NewCmd(env Env) (func() *exec.Cmd, error)
}