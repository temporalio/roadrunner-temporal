package resetter

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

// Reset named service.
func (rpc *rpc) Reset(service string, done *bool) error {
	*done = true
	return rpc.srv.Reset(service)
}
