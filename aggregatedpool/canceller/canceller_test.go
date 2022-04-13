package canceller

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_CancellerNoListeners(t *testing.T) {
	c := &Canceller{}

	assert.NoError(t, c.Cancel(1))
}

func Test_CancellerListenerError(t *testing.T) {
	c := &Canceller{}
	c.Register(1, func() error {
		return errors.New("failed")
	})

	assert.Error(t, c.Cancel(1))
}

func Test_CancellerListenerDiscarded(t *testing.T) {
	c := &Canceller{}
	c.Register(1, func() error {
		return errors.New("failed")
	})

	c.Discard(1)
	assert.NoError(t, c.Cancel(1))
}
