package test

import (
	"fmt"
	"testing"

	"github.com/spiral/endure"
	"github.com/spiral/roadrunner/v2/plugins/config"
	"github.com/spiral/roadrunner/v2/plugins/factory"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
)

func TestGeneral(t *testing.T) {
	cont, err := endure.NewContainer(endure.DebugLevel, endure.RetryOnFail(true))
	if err != nil {
		t.Fatal(err)
	}

	conf := &config.ViperProvider{
		Path:   ".rr.yaml",
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

	err = cont.Register(&temporal.Server{})
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
	// todo add signal to stop

	for {
		select {
		case e := <-errCh:
			fmt.Println(e)
		}
	}
}
