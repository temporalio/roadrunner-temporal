package aggregatedpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/temporalio/roadrunner-temporal/v5/api"
	sdkinterceptor "go.temporal.io/sdk/interceptor"
)

// mockInterceptor is a minimal api.Interceptor for testing.
type mockInterceptor struct {
	name string
	wi   sdkinterceptor.WorkerInterceptor
}

func (m *mockInterceptor) Name() string                                        { return m.name }
func (m *mockInterceptor) WorkerInterceptor() sdkinterceptor.WorkerInterceptor { return m.wi }

// taggedWorkerInterceptor embeds the base and carries a tag so we can identify it.
type taggedWorkerInterceptor struct {
	sdkinterceptor.WorkerInterceptorBase
	tag string
}

func newMock(name string) (*mockInterceptor, *taggedWorkerInterceptor) {
	wi := &taggedWorkerInterceptor{tag: name}
	return &mockInterceptor{name: name, wi: wi}, wi
}

func TestResolveInterceptors_EmptyConfig_ReturnsAll(t *testing.T) {
	mockA, wiA := newMock("a")
	mockB, wiB := newMock("b")

	interceptors := map[string]api.Interceptor{
		"a": mockA,
		"b": mockB,
	}

	result, err := ResolveInterceptors(interceptors, nil)
	require.NoError(t, err)

	// first element is always the built-in interceptor
	assert.Len(t, result, 3)

	// collect tags from non-builtin interceptors (positions 1+)
	tags := make(map[string]bool, 2)
	for _, wi := range result[1:] {
		tagged, ok := wi.(*taggedWorkerInterceptor)
		require.True(t, ok)
		tags[tagged.tag] = true
	}

	assert.True(t, tags[wiA.tag])
	assert.True(t, tags[wiB.tag])
}

func TestResolveInterceptors_SpecifiedOrder(t *testing.T) {
	mockA, wiA := newMock("a")
	mockB, wiB := newMock("b")
	mockC, wiC := newMock("c")

	interceptors := map[string]api.Interceptor{
		"a": mockA,
		"b": mockB,
		"c": mockC,
	}

	result, err := ResolveInterceptors(interceptors, []string{"c", "a", "b"})
	require.NoError(t, err)
	require.Len(t, result, 4) // built-in + 3

	// verify exact order after the built-in
	assert.Equal(t, wiC, result[1])
	assert.Equal(t, wiA, result[2])
	assert.Equal(t, wiB, result[3])
}

func TestResolveInterceptors_UnknownName_ReturnsError(t *testing.T) {
	mockA, _ := newMock("a")

	interceptors := map[string]api.Interceptor{
		"a": mockA,
	}

	_, err := ResolveInterceptors(interceptors, []string{"a", "nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"nonexistent"`)
}

func TestResolveInterceptors_EmptyMap_EmptyConfig(t *testing.T) {
	result, err := ResolveInterceptors(map[string]api.Interceptor{}, nil)
	require.NoError(t, err)

	// only the built-in interceptor
	assert.Len(t, result, 1)
}
