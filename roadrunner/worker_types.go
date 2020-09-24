package roadrunner

// todo: implement
type AsyncWorker interface {
	OnReceive(func())
}
