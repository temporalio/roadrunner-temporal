package informer

import "github.com/spiral/roadrunner/v2"

type rpc struct {
	srv *Plugin
}

// List all resettable services.
func (rpc *rpc) List(request bool, list *[]string) error {
	*list = make([]string, 0)

	for name := range rpc.srv.registry {
		*list = append(*list, name)
	}

	return nil
}

// WorkerList contains list of workers.
type WorkerList struct {
	// Workers is list of workers.
	Workers []roadrunner.ProcessState `json:"workers"`
}

// Workers state of a given service.
func (rpc *rpc) Workers(service string, list *WorkerList) error {
	workers, err := rpc.srv.Workers(service)
	if err != nil {
		return err
	}

	list.Workers = make([]roadrunner.ProcessState, 0)
	for _, w := range workers {
		ps, err := roadrunner.WorkerProcessState(w)
		if err != nil {
			continue
		}

		list.Workers = append(list.Workers, ps)
	}

	return nil
}
