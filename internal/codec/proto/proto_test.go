package proto

import (
	"log/slog"
	"testing"
	"time"

	protocolV1 "github.com/roadrunner-server/api-go/v6/temporal/v1"
	"github.com/roadrunner-server/pool/v2/payload"
	"github.com/stretchr/testify/require"
	"github.com/temporalio/roadrunner-temporal/v6/internal"
	"go.temporal.io/sdk/converter"
	"google.golang.org/protobuf/proto"
)

// The PHP SDK wire format uses encoding/json v1 semantics.
func TestDecodeCommandOptions(t *testing.T) {
	tests := []struct {
		name    string
		options string
		want    internal.ExecuteActivity
	}{
		{
			name:    "duration as nanoseconds",
			options: `{"name":"a","options":{"TaskQueueName":"q","ScheduleToCloseTimeout":1000000000}}`,
			want:    internal.ExecuteActivity{Name: "a"},
		},
		{
			name:    "case-insensitive field names",
			options: `{"Name":"a","options":{"taskqueuename":"q","scheduletoclosetimeout":1000000000}}`,
			want:    internal.ExecuteActivity{Name: "a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.want.Options.TaskQueueName = "q"
			tt.want.Options.ScheduleToCloseTimeout = time.Second

			body, err := proto.Marshal(&protocolV1.Frame{Messages: []*protocolV1.Message{
				{Command: "ExecuteActivity", Options: []byte(tt.options)},
			}})
			require.NoError(t, err)

			var out []*internal.Message
			c := NewCodec(slog.Default(), converter.GetDefaultDataConverter())
			require.NoError(t, c.Decode(&payload.Payload{Body: body}, &out))
			require.Len(t, out, 1)
			require.Equal(t, &tt.want, out[0].Command)
		})
	}
}

func TestEncodeContextOmitsEmptyFields(t *testing.T) {
	p := &payload.Payload{}
	c := NewCodec(slog.Default(), converter.GetDefaultDataConverter())
	require.NoError(t, c.Encode(&internal.Context{}, p, &internal.Message{Command: internal.GetWorkerInfo{}}))
	require.JSONEq(t, `{"rr_id":"","continue_as_new_suggested":false}`, string(p.Context))
}
