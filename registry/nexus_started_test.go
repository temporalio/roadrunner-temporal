package registry

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNexusStartedRegistry_PushThenListen(t *testing.T) {
	r := &NexusStartedRegistry{}

	r.Push(7, "tok-async", nil)

	var gotToken string
	var gotErr error
	r.Listen(7, func(token string, err error) {
		gotToken = token
		gotErr = err
	})

	assert.Equal(t, "tok-async", gotToken)
	assert.NoError(t, gotErr)
}

func TestNexusStartedRegistry_ListenThenPush(t *testing.T) {
	r := &NexusStartedRegistry{}

	var gotToken string
	var gotErr error
	r.Listen(11, func(token string, err error) {
		gotToken = token
		gotErr = err
	})
	assert.Empty(t, gotToken, "listener must not fire before Push")

	r.Push(11, "late-tok", nil)

	assert.Equal(t, "late-tok", gotToken)
	assert.NoError(t, gotErr)
}

// Sync Nexus ops push token="" — listener must still fire so caller resolves cleanly.
func TestNexusStartedRegistry_EmptyTokenSync(t *testing.T) {
	r := &NexusStartedRegistry{}

	var fired bool
	var gotToken string
	r.Listen(3, func(token string, err error) {
		fired = true
		gotToken = token
	})
	r.Push(3, "", nil)

	assert.True(t, fired)
	assert.Equal(t, "", gotToken)
}

func TestNexusStartedRegistry_PushError(t *testing.T) {
	r := &NexusStartedRegistry{}

	startErr := errors.New("nexus start failed")
	var gotErr error
	var gotToken string
	r.Listen(42, func(token string, err error) {
		gotToken = token
		gotErr = err
	})
	r.Push(42, "", startErr)

	assert.Equal(t, "", gotToken)
	assert.Same(t, startErr, gotErr)
}

func TestNexusStartedRegistry_PushErrorBeforeListen(t *testing.T) {
	r := &NexusStartedRegistry{}

	startErr := errors.New("nexus start failed")
	r.Push(42, "", startErr)

	var gotErr error
	r.Listen(42, func(token string, err error) {
		gotErr = err
	})

	assert.Same(t, startErr, gotErr)
}

func TestNexusStartedRegistry_DistinctIDs(t *testing.T) {
	r := &NexusStartedRegistry{}

	var firedA, firedB bool
	r.Listen(1, func(string, error) { firedA = true })
	r.Listen(2, func(string, error) { firedB = true })

	r.Push(1, "tA", nil)
	assert.True(t, firedA)
	assert.False(t, firedB, "listener for ID 2 must not fire on Push(1, …)")

	r.Push(2, "tB", nil)
	assert.True(t, firedB)
}

// Repeated Push for the same ID overwrites — late Listen sees only the latest entry.
func TestNexusStartedRegistry_OverwriteEntry(t *testing.T) {
	r := &NexusStartedRegistry{}

	r.Push(5, "first", nil)
	r.Push(5, "second", nil)

	var gotToken string
	r.Listen(5, func(token string, err error) {
		gotToken = token
	})
	assert.Equal(t, "second", gotToken)
}

// Second Listen for the same ID wins — Push must reach only the most recent listener.
func TestNexusStartedRegistry_ListenReplacesListener(t *testing.T) {
	r := &NexusStartedRegistry{}

	var firedFirst, firedSecond bool
	r.Listen(8, func(string, error) { firedFirst = true })
	r.Listen(8, func(string, error) { firedSecond = true })

	r.Push(8, "tok", nil)

	assert.False(t, firedFirst, "replaced listener must not fire")
	assert.True(t, firedSecond)
}
