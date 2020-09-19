package config

type ViperProvider struct {
}

//////// ENDURE //////////
func (v *ViperProvider) Init() error {
	return nil
}

func (v *ViperProvider) Configure() error {
	return nil
}

func (v *ViperProvider) Close() error {
	return nil
}

func (v *ViperProvider) Provides() []interface{} {
	return []interface{}{
		v.Logger,
	}
}

func (v *ViperProvider) Logger() (*ViperProvider, error) {
	return v, nil
}

func (v *ViperProvider) Serve() chan error {
	return nil
}

func (v *ViperProvider) Stop() error {
	return nil
}

///////////// VIPER ///////////////

func (v *ViperProvider) SetPath(name string) error {
	return nil
}

func (v *ViperProvider) Overwrite(values map[string]string) error {
	//https://github.com/spiral/roadrunner/blob/master/cmd/util/config.go#L109
	return nil
}

func (v *ViperProvider) Unmarshal(name string, out interface{}) error {
	return nil
}

// Get raw config in a form of config section.
func (v *ViperProvider) Get(name string) (Section, error) {
	return nil, nil
}
