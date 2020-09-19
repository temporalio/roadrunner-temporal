package config

import (
	"testing"

	"github.com/spiral/endure"
)

func TestViperProvider_Init(t *testing.T) {
	c, err := endure.NewContainer(endure.DebugLevel, endure.RetryOnFail(true))
	if err != nil {
		t.Fatal(err)
	}
	err = c.Register(&ViperProvider{})
	if err != nil {
		t.Fatal(err)
	}
}
