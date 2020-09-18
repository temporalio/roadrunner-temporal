package config

type Provider interface {
	// Unmarshal configuration section into configuration object.
	//
	// func (h *HttpService) Init(cp config.Provider) error {
	//     h.config := &HttpConfig{}
	//     if err := configProvider.Unmarshal("http", h.config); err != nil {
	//         return err
	//     }
	// }
	Unmarshal(name string, out interface{}) error

	// Get raw config in a form of config section.
	Get(name string) (Section, error)
}

type Section interface {
	Keys() []string
	Get(name string) (interface{}, error)
}

// todo: implement service based on viper config
// todo: see RR implementation + ENV + merge configs
