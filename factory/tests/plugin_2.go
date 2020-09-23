package tests

import (
	"errors"
	"fmt"

	"github.com/temporalio/roadrunner-temporal/config"
	"github.com/temporalio/roadrunner-temporal/factory"
)

type Foo2 struct {
	configProvider  config.Provider
	factoryProvider factory.Provider
}

func (f *Foo2) Init(p config.Provider, app factory.Provider) error {
	f.configProvider = p
	f.factoryProvider = app
	return nil
}

func (f *Foo2) Serve() chan error {
	errCh := make(chan error, 1)

	err := f.configProvider.SetPath(".rr_2.yaml")
	if err != nil {
		errCh <- err
		return errCh
	}

	r := &factory.AppConfig{}
	err = f.configProvider.UnmarshalKey("app", r)
	if err != nil {
		errCh <- err
		return errCh
	}

	cmd, err := f.factoryProvider.NewCmd(nil)
	if err != nil {
		errCh <- err
		return errCh
	}
	if cmd == nil {
		errCh <- errors.New("command is nil")
		return errCh
	}
	a := cmd()
	out, err := a.Output()
	if err != nil {
		errCh <- err
		return errCh
	}

	fmt.Println(string(out))

	return errCh
}

func (f *Foo2) Stop() error {
	return nil
}
