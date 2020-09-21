package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type ViperProvider struct {
	viper *viper.Viper
}

//////// ENDURE //////////
func (v *ViperProvider) Init() error {
	v.viper = viper.New()
	// read in environment variables that match
	v.viper.AutomaticEnv()
	v.viper.SetEnvPrefix("rr")
	v.viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	return nil
}

func (v *ViperProvider) Provides() []interface{} {
	return []interface{}{
		v.Logger,
	}
}

// Provide config Provider dep
func (v *ViperProvider) Logger() (Provider, error) {
	return v, nil
}

func (v *ViperProvider) Serve() chan error {
	errCh := make(chan error, 1)
	return errCh
}

func (v *ViperProvider) Stop() error {
	v.viper = nil
	return nil
}

///////////// VIPER ///////////////

// AddConfigPath adds a path for Viper to search for the config file in.
// Can be called multiple times to define multiple search paths.
// Also reads config provided by path
func (v *ViperProvider) SetPath(name string) error {
	v.viper.SetConfigFile(name)
	err := v.viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
	return nil
}

// Overwrite overwrites existing config with provided values
func (v *ViperProvider) Overwrite(values map[string]string) error {
	if len(values) != 0 {
		for _, flag := range values {
			key, value, err := parseFlag(flag)
			if err != nil {
				return err
			}
			v.viper.Set(key, value)
		}
	}

	return nil
}

//
func (v *ViperProvider) UnmarshalKey(name string, out interface{}) error {
	err := v.viper.UnmarshalKey(name, &out)
	if err != nil {
		return err
	}
	return nil
}

// Get raw config in a form of config section.
func (v *ViperProvider) Get(name string) interface{} {
	return v.viper.Get(name)
}

/////////// PRIVATE //////////////

func parseFlag(flag string) (string, string, error) {
	if !strings.Contains(flag, "=") {
		return "", "", fmt.Errorf("invalid flag `%s`", flag)
	}

	parts := strings.SplitN(strings.TrimLeft(flag, " \"'`"), "=", 2)

	return strings.Trim(parts[0], " \n\t"), parseValue(strings.Trim(parts[1], " \n\t")), nil
}

func parseValue(value string) string {
	escape := []rune(value)[0]

	if escape == '"' || escape == '\'' || escape == '`' {
		value = strings.Trim(value, string(escape))
		value = strings.Replace(value, fmt.Sprintf("\\%s", string(escape)), string(escape), -1)
	}

	return value
}
