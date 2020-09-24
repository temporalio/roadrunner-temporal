package roadrunner

// TaskError is job level error (no worker halt), wraps at top
// of error context
type TaskError []byte

// Error converts error context to string
func (te TaskError) Error() string {
	return string(te)
}

// WorkerError is worker related error
type WorkerError struct {
	// Worker
	Worker *Worker

	// Caused error
	Caused error
}

// Error converts error context to string
func (e WorkerError) Error() string {
	return e.Caused.Error()
}
