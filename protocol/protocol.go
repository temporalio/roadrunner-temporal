package roadrunner_temporal //nolint:golint,stylecheck

import (
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

		// Error associated with command id.
		Error *Error `json:"error,omitempty"`

		// Payloads contains message specific payloads in binary format.
		Payloads *commonpb.Payloads `json:"payloads,omitempty"`
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

	// Endpoint provides the ability to send and receive messages.
	Endpoint interface {
		// ExecWithContext allow to set ExecTTL
		Exec(p payload.Payload) (payload.Payload, error)
	}

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

// Error returns error as string.
func (e Error) Error() string {
	return e.Message
}
