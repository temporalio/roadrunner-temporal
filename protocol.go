package roadrunner_temporal

import (
	"github.com/fatih/color"
	jsoniter "github.com/json-iterator/go"
	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/pkg/payload"

	"log"
	"time"
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

		// Command name. Optional.
		Command string `json:"command,omitempty"`

		// Command parameters (free form).
		Params jsoniter.RawMessage `json:"params,omitempty"`

		// Result always contains array of values.
		Result []jsoniter.RawMessage `json:"result,omitempty"`

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
)

// IsEmpty only check if task queue set.
func (ctx Context) IsEmpty() bool {
	return ctx.TaskQueue == ""
}

// IsCommand returns true if message carries request.
func (msg Message) IsCommand() bool {
	return msg.Command != ""
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

// Exchange commands with worker.
func Execute(e Endpoint, ctx Context, msg ...Message) ([]Message, error) {
	if len(msg) == 0 {
		return nil, nil
	}

	var (
		result = make([]Message, 0, 5)
		err    error
	)

	p := payload.Payload{}

	if ctx.IsEmpty() {
		p.Context = []byte("null")
	}

	p.Context, err = jsoniter.Marshal(ctx)
	if err != nil {
		return nil, errors.E(errors.Op("encodeContext"), err)
	}

	p.Body, err = jsoniter.Marshal(msg)
	if err != nil {
		return nil, errors.E(errors.Op("encodePayload"), err)
	}

	// todo: REMOVE once complete
	log.Print(color.GreenString(string(p.Body)))

	out, err := e.Exec(p)
	if err != nil {
		return nil, errors.E(errors.Op("execute"), err)
	}

	// todo: REMOVE once complete
	log.Print(color.HiYellowString(string(out.Body)))

	err = jsoniter.Unmarshal(out.Body, &result)
	if err != nil {
		return nil, errors.E(errors.Op("parseResponse"), err)
	}

	return result, nil
}
