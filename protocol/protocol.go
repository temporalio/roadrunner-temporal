package roadrunner_temporal //nolint:golint,stylecheck

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/spiral/roadrunner/v2/pkg/payload"
	"github.com/spiral/roadrunner/v2/plugins/logger"
	commonpb "go.temporal.io/api/common/v1"
	"time"
)

const (
	// DebugNone disables all debug messages.
	DebugNone = iota

	// DebugNormal renders all messages into console.
	DebugNormal

	// DebugHumanized enables color highlights for messages.
	DebugHumanized
)

type (
	// Endpoint provides the ability to send and receive messages.
	Endpoint interface {
		// ExecWithContext allow to set ExecTTL
		Exec(p payload.Payload) (payload.Payload, error)
	}

	// Context provides worker information about currently. Context can be empty for server level commands.
	Context struct {
		// TaskQueue associates message batch with the specific task queue in underlying worker.
		TaskQueue string `json:"taskQueue,omitempty"`

		// TickTime associated current or historical time with message batch.
		TickTime time.Time `json:"tickTime,omitempty"`

		// Replay indicates that current message batch is historical.
		Replay bool `json:"replay,omitempty"`
	}

	// Messages used to exchange the send commands and receive responses from underlying workers.
	Message struct {
		// ID contains ID of the command, response or error.
		ID uint64 `json:"id"`

		// Command of the message in unmarshalled form. Pointer.
		Command interface{} `json:"params,omitempty"`

		// Result always contains array of values.
		Result []*commonpb.Payload `json:"result,omitempty"`

		// Error associated with command id.
		Error *Error `json:"error,omitempty"`
	}

	// Error from underlying worker.
	Error struct {
		// Code of the error.
		Code int `json:"code"`

		// Message contains exception message.
		Message string `json:"message"`

		// Data contains additional error context. Typically stack trace.
		Data interface{} `json:"data"`
	}

	// Codec manages payload encoding and decoding while communication with underlying worker.
	Codec interface {
		// WithLogger creates new codes instance with attached logger.
		WithLogger(logger.Logger) Codec

		// GetName returns codec name.
		GetName() string

		// Execute sends message to worker and waits for the response.
		Execute(e Endpoint, ctx Context, msg ...Message) ([]Message, error)
	}

	// Encoder encodes messages sent to worker.
	Encoder func(interface{}) ([]byte, error)

	// Decoder decodes messages sent to worker.
	Decoder func([]byte, interface{}) error

	// DebugLevel configures debug level.
	DebugLevel int
)

// IsEmpty only check if task queue set.
func (ctx Context) IsEmpty() bool {
	return ctx.TaskQueue == ""
}

// IsCommand returns true if message carries request.
func (msg Message) IsCommand() bool {
	return msg.Command != nil
}

// IsError returns true if message carries error.
func (msg Message) IsError() bool {
	return msg.Error != nil
}

// String converts message into string.
func (msg Message) String() string {
	data, err := jsoniter.Marshal(msg)
	if err != nil {
		return err.Error()
	}

	return string(data)
}

// Error returns error as string.
func (e Error) Error() string {
	return e.Message
}
