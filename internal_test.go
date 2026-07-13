package rrtemporal

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectError_IncludesAddressCauseAndHint(t *testing.T) {
	cause := errors.New("context deadline exceeded")
	err := connectError("localhost:7233", cause)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "localhost:7233", "should name the target address")
	assert.Contains(t, err.Error(), "context deadline exceeded", "should include the underlying cause")
	assert.Contains(t, err.Error(), "reachable", "should hint about reachability/readiness")
	assert.ErrorIs(t, err, cause, "should wrap the cause so errors.Is keeps working")
}
