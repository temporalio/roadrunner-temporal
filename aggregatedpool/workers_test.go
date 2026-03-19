package aggregatedpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/temporalio/roadrunner-temporal/v5/api"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	sdkinterceptor "go.temporal.io/sdk/interceptor"
)

// mockPayloadConverter implements converter.PayloadConverter for testing.
type mockPayloadConverter struct {
	encoding string
}

func (m *mockPayloadConverter) ToPayload(any) (*commonpb.Payload, error) { return nil, nil }
func (m *mockPayloadConverter) FromPayload(*commonpb.Payload, any) error { return nil }
func (m *mockPayloadConverter) ToString(*commonpb.Payload) string        { return "" }
func (m *mockPayloadConverter) Encoding() string                         { return m.encoding }

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

func TestResolveInterceptors_EnabledSubset(t *testing.T) {
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

func TestResolveInterceptors_EnabledSubset_FiltersUnlisted(t *testing.T) {
	mockA, wiA := newMock("a")
	mockB, _ := newMock("b")
	mockC, wiC := newMock("c")

	interceptors := map[string]api.Interceptor{
		"a": mockA,
		"b": mockB,
		"c": mockC,
	}

	// register a, b, c — enable only a, c → only a, c returned
	result, err := ResolveInterceptors(interceptors, []string{"a", "c"})
	require.NoError(t, err)
	require.Len(t, result, 3) // built-in + 2

	assert.Equal(t, wiA, result[1])
	assert.Equal(t, wiC, result[2])
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

func TestResolveInterceptors_NilMap_NilConfig(t *testing.T) {
	result, err := ResolveInterceptors(nil, nil)
	require.NoError(t, err)

	// only the built-in interceptor
	assert.Len(t, result, 1)
}

func TestResolveInterceptors_NilMap_WithConfig(t *testing.T) {
	_, err := ResolveInterceptors(nil, []string{"a"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"a"`)
}

func TestResolveInterceptors_DuplicateNames(t *testing.T) {
	mockA, _ := newMock("a")

	interceptors := map[string]api.Interceptor{
		"a": mockA,
	}

	result, err := ResolveInterceptors(interceptors, []string{"a", "a"})
	require.NoError(t, err)

	// built-in + 2 copies of "a"
	assert.Len(t, result, 3)
}

func TestResolveDataConverters_EmptyConfig_ReturnsAll(t *testing.T) {
	dcA := &mockPayloadConverter{encoding: "encoding/a"}
	dcB := &mockPayloadConverter{encoding: "encoding/b"}

	converters := map[string]converter.PayloadConverter{
		"encoding/a": dcA,
		"encoding/b": dcB,
	}

	result, err := ResolveDataConverters(converters, nil)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	encodings := make(map[string]bool, 2)
	for _, dc := range result {
		encodings[dc.Encoding()] = true
	}
	assert.True(t, encodings["encoding/a"])
	assert.True(t, encodings["encoding/b"])
}

func TestResolveDataConverters_EnabledSubset(t *testing.T) {
	dcA := &mockPayloadConverter{encoding: "encoding/a"}
	dcB := &mockPayloadConverter{encoding: "encoding/b"}
	dcC := &mockPayloadConverter{encoding: "encoding/c"}

	converters := map[string]converter.PayloadConverter{
		"encoding/a": dcA,
		"encoding/b": dcB,
		"encoding/c": dcC,
	}

	result, err := ResolveDataConverters(converters, []string{"encoding/c", "encoding/a", "encoding/b"})
	require.NoError(t, err)
	require.Len(t, result, 3)

	assert.Equal(t, dcC, result[0])
	assert.Equal(t, dcA, result[1])
	assert.Equal(t, dcB, result[2])
}

func TestResolveDataConverters_EnabledSubset_FiltersUnlisted(t *testing.T) {
	dcA := &mockPayloadConverter{encoding: "encoding/a"}
	dcB := &mockPayloadConverter{encoding: "encoding/b"}
	dcC := &mockPayloadConverter{encoding: "encoding/c"}

	converters := map[string]converter.PayloadConverter{
		"encoding/a": dcA,
		"encoding/b": dcB,
		"encoding/c": dcC,
	}

	// register a, b, c — enable only a, c → only a, c returned
	result, err := ResolveDataConverters(converters, []string{"encoding/a", "encoding/c"})
	require.NoError(t, err)
	require.Len(t, result, 2)

	assert.Equal(t, dcA, result[0])
	assert.Equal(t, dcC, result[1])
}

func TestResolveDataConverters_UnknownEncoding_ReturnsError(t *testing.T) {
	dcA := &mockPayloadConverter{encoding: "encoding/a"}

	converters := map[string]converter.PayloadConverter{
		"encoding/a": dcA,
	}

	_, err := ResolveDataConverters(converters, []string{"encoding/a", "encoding/nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"encoding/nonexistent"`)
}

func TestResolveDataConverters_EmptyMap_EmptyConfig(t *testing.T) {
	result, err := ResolveDataConverters(map[string]converter.PayloadConverter{}, nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestResolveDataConverters_NilMap_NilConfig(t *testing.T) {
	result, err := ResolveDataConverters(nil, nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestResolveDataConverters_NilMap_WithConfig(t *testing.T) {
	_, err := ResolveDataConverters(nil, []string{"encoding/a"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"encoding/a"`)
}

func TestResolveDataConverters_EmptyMap_WithConfig(t *testing.T) {
	_, err := ResolveDataConverters(map[string]converter.PayloadConverter{}, []string{"encoding/a"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"encoding/a"`)
}

func TestResolveDataConverters_DuplicateEncodings(t *testing.T) {
	dcA := &mockPayloadConverter{encoding: "encoding/a"}

	converters := map[string]converter.PayloadConverter{
		"encoding/a": dcA,
	}

	result, err := ResolveDataConverters(converters, []string{"encoding/a", "encoding/a"})
	require.NoError(t, err)
	assert.Len(t, result, 2)
}
