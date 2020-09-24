package roadrunner

import "os/exec"

// Factory is responsible of wrapping given command into tasks WorkerProcess.
type Factory interface {
	// SpawnWorker creates new WorkerProcess process based on given command.
	// Process must not be started.
	SpawnWorker(cmd *exec.Cmd) (w Worker, err error)

	// Close the factory and underlying connections.
	Close() error
}
