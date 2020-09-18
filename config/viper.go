package config

type ViperProvider struct {
}

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
