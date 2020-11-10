package resetter

type rpc struct {
	srv *Plugin
}

func (rpc *rpc) List(request bool, list *[]string) error {
	*list = make([]string, 0)
	*list = append(*list, "HELLO")

	// todo: implement

	return nil
}
