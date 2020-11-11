package informer

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
