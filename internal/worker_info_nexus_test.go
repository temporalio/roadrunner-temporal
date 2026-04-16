package internal

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerInfo_NexusServicesJSON(t *testing.T) {
	// Verify that PHP's "nexusServices" JSON key is properly parsed
	// and Operations are correctly unmarshaled.
	jsonPayload := []byte(`{
		"TaskQueue": "test-queue",
		"nexusServices": [
			{"name": "GreetingService", "operations": ["greet", "farewell"]},
			{"name": "EchoService", "operations": ["echo"]}
		]
	}`)

	var wi WorkerInfo
	require.NoError(t, json.Unmarshal(jsonPayload, &wi))

	assert.Equal(t, "test-queue", wi.TaskQueue)
	assert.Len(t, wi.NexusServices, 2)

	assert.Equal(t, "GreetingService", wi.NexusServices[0].Name)
	assert.Equal(t, []string{"greet", "farewell"}, wi.NexusServices[0].Operations)

	assert.Equal(t, "EchoService", wi.NexusServices[1].Name)
	assert.Equal(t, []string{"echo"}, wi.NexusServices[1].Operations)
}

func TestWorkerInfo_EmptyNexusServices(t *testing.T) {
	// Verify that missing or empty nexusServices field is handled gracefully
	jsonPayload := []byte(`{"TaskQueue": "test-queue"}`)

	var wi WorkerInfo
	require.NoError(t, json.Unmarshal(jsonPayload, &wi))

	assert.Equal(t, "test-queue", wi.TaskQueue)
	assert.Empty(t, wi.NexusServices)
}

func TestNexusServiceInfo_JSONMarshal(t *testing.T) {
	svc := NexusServiceInfo{
		Name:       "GreetingService",
		Operations: []string{"greet"},
	}

	data, err := json.Marshal(svc)
	require.NoError(t, err)

	assert.JSONEq(t, `{"name":"GreetingService","operations":["greet"]}`, string(data))
}

// TestWorkerInfo_NexusServicesJSONLowercase guards Go's case-insensitive
// JSON unmarshal: PHP may ship `nexusServices` (camelCase) and Go must still
// populate the PascalCase-tagged field.
func TestWorkerInfo_NexusServicesJSONLowercase(t *testing.T) {
	jsonPayload := []byte(`{"nexusServices":[{"name":"S","operations":["o"]}]}`)
	var wi WorkerInfo
	require.NoError(t, json.Unmarshal(jsonPayload, &wi))
	require.Len(t, wi.NexusServices, 1)
	assert.Equal(t, "S", wi.NexusServices[0].Name)
}

func TestWorkerInfo_HasFlag(t *testing.T) {
	for name, tc := range map[string]struct {
		flags map[string]string
		key   string
		want  bool
	}{
		"missing":           {nil, "x", false},
		"absent":            {map[string]string{"y": "true"}, "x", false},
		"true lowercase":    {map[string]string{"x": "true"}, "x", true},
		"True mixedcase":    {map[string]string{"x": "True"}, "x", true},
		"TRUE uppercase":    {map[string]string{"x": "TRUE"}, "x", true},
		"one":               {map[string]string{"x": "1"}, "x", true},
		"empty value false": {map[string]string{"x": ""}, "x", false},
		"false":             {map[string]string{"x": "false"}, "x", false},
		"zero":              {map[string]string{"x": "0"}, "x", false},
		"yes (rejected)":    {map[string]string{"x": "yes"}, "x", false},
	} {
		t.Run(name, func(t *testing.T) {
			wi := WorkerInfo{Flags: tc.flags}
			assert.Equal(t, tc.want, wi.HasFlag(tc.key))
		})
	}
}
