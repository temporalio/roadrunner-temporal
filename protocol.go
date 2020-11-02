package roadrunner_temporal

import (
	"encoding/json"
	"log"
	"time"

	"github.com/fatih/color"
	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2"
)

// Endpoint provides the ability to send and receive messages.
type Endpoint interface {
	// ExecWithContext allow to set ExecTTL
	Exec(p roadrunner.Payload) (roadrunner.Payload, error)
}

// Context provides worker information about currently. Context can be empty for server level commands.
type Context struct {
	// TaskQueue associates message batch with the specific task queue in underlying worker.
	TaskQueue string `json:"taskQueue,omitempty"`

	// TickTime associated current or historical time with message batch.
	TickTime time.Time `json:"tickTime,omitempty"`

	// Replay indicates that current message batch is historical.
	Replay bool `json:"replay,omitempty"`
}

// IsEmpty only check if task queue set.
func (ctx Context) IsEmpty() bool {
	return ctx.TaskQueue == ""
}

// Messages used to exchange the send commands and receive responses from underlying workers.
type Message struct {
	// ID contains ID of the command, response or error.
	ID uint64 `json:"id"`

	// Command name. Optional.
	Command string `json:"command,omitempty"`

	// Command parameters (free form).
	Params json.RawMessage `json:"params,omitempty"`

	// Result always contains array of values.
	Result []json.RawMessage `json:"result"`

	// Error associated with command id.
	Error interface{} `json:"error,omitempty"`
}

// Error from underlying worker. todo: implement
type Error struct {
	// Code of the error.
	Code int `json:"code"`

	// Message contains exception message.
	Message string `json:"message"`

	// Data contains additional error context.
	Data interface{} `json:"data"`
}

// String converts message into string.
func (msg Message) String() string {
	data, err := json.Marshal(msg)
	if err != nil {
		return err.Error()
	}

	return string(data)
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

	p := roadrunner.Payload{}

	if ctx.IsEmpty() {
		p.Context = []byte("null")
	}

	p.Context, err = json.Marshal(ctx)
	if err != nil {
		return nil, errors.E(errors.Op("encodeContext"), err)
	}

	p.Body, err = json.Marshal(msg)
	if err != nil {
		return nil, errors.E(errors.Op("encodePayload"), err)
	}

	// todo: debug flag?
	log.Print(color.GreenString(string(p.Body)))

	out, err := e.Exec(p)
	if err != nil {
		return nil, errors.E(errors.Op("execute"), err)
	}

	log.Print(color.HiYellowString(string(out.Body)))

	err = json.Unmarshal(out.Body, &result)
	if err != nil {
		return nil, errors.E(errors.Op("parseResponse"), err)
	}

	return result, nil
}
