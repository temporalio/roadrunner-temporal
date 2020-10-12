Worker states

// EventWorkerError triggered after WorkerProcess. Except payload to be error.
EventWorkerError int64 = iota + 100

// EventWorkerLog triggered on every write to WorkerProcess StdErr pipe (batched). Except payload to be []byte string.
EventWorkerLog

// EventWorkerWaitDone triggered when worker exit from process Wait
EventWorkerWaitDone

EventWorkerBufferClosed

EventRelayCloseError

EventWorkerProcessError

EventWorkerRemove -- if worker currently processing the request it is not present in a stack. We mark it with this state 
to remove on GetFreeWorker stage.