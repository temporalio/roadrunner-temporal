package config

type Provider interface {
	// Unmarshal configuration section into configuration object.
	//
	// func (h *HttpService) Init(cp config.Provider) error {
	//     h.config := &HttpConfig{}
	//     if err := configProvider.UnmarshalKey("http", h.config); err != nil {
	//         return err
	//     }
	// }
	UnmarshalKey(name string, out interface{}) error
	// SetPath sets search path for the config
	SetPath(name string) error
	// Get raw config in a form of config section.
	// TODO first candidate to delete
	Get(name string) interface{}
}

// todo: implement service based on viper config
// todo: see RR implementation + ENV + merge configs
