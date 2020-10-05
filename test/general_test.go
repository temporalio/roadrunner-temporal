package test

import (
	"fmt"
	"testing"

	"github.com/spiral/endure"
	"github.com/temporalio/roadrunner-temporal/config"
	"github.com/temporalio/roadrunner-temporal/factory"
)

func TestGeneral(t *testing.T) {
	cont, err := endure.NewContainer(endure.DebugLevel, endure.RetryOnFail(true))
	if err != nil {
		t.Fatal(err)
	}

	conf := &config.ViperProvider{
		Path:   ".",
		Prefix: "rr",
	}
	err = cont.Register(conf)
	if err != nil {
		t.Fatal(err)
	}

	err = cont.Register(&factory.App{})
	if err != nil {
		t.Fatal(err)
	}

	err = cont.Register(&factory.WFactory{})
	if err != nil {
		t.Fatal(err)
	}

	err = cont.Init()
	if err != nil {
		t.Fatal(err)
	}

	errCh, err := cont.Serve()
	if err != nil {
		t.Fatal(err)
	}

	for {
		select {
		case e := <-errCh:
			fmt.Println(e)
		}
	}
}
