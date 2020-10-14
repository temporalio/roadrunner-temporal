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

	// Has checks if config key exists.
	Has(name string) bool

	// Get used to get config section
	Get(name string) interface{}
}
