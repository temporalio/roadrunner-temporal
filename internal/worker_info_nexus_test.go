package internal

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerInfo_NexusServicesJSON(t *testing.T) {
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
