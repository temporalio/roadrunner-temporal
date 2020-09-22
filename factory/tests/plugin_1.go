package tests

import (
	"errors"
	"fmt"

	"github.com/temporalio/roadrunner-temporal/config"
	"github.com/temporalio/roadrunner-temporal/factory"
)

type Foo struct {
	configProvider  config.Provider
	factoryProvider factory.Provider
}

func (f *Foo) Init(p config.Provider, app factory.Provider) error {
	f.configProvider = p
	f.factoryProvider = app
	return nil
}

func (f *Foo) Serve() chan error {
	errCh := make(chan error, 1)

	err := f.configProvider.SetPath(".rr.yaml")
	if err != nil {
		errCh <- err
	}

	r := &factory.AppConfig{}
	err = f.configProvider.UnmarshalKey("app", r)
	if err != nil {
		errCh <- err
	}

	cmd, err := f.factoryProvider.NewCmd(nil)
	if err != nil {
		errCh <- err
	}
	if cmd == nil {
		errCh <- errors.New("command is nil")
		return errCh
	}

	a := cmd()
	if err != nil {
		errCh <- err
	}
	out, err := a.Output()
	if err != nil {
		errCh <- err
		return errCh
	}

	fmt.Println(string(out))

	return errCh
}

func (f *Foo) Stop() error {
	return nil
}
