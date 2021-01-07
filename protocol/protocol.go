package roadrunner_temporal //nolint:golint,stylecheck

import (
	"encoding/json"
	"github.com/fatih/color"
	jsoniter "github.com/json-iterator/go"
	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/pkg/payload"
	commonpb "go.temporal.io/api/common/v1"
	"log"
	"time"
)

const (
	// DebugMode enabled verbose debug of communication protocol.
	DebugMode = false
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

	// messageFrame contains message command in binary form.
	messageFrame struct {
		// ID contains ID of the command, response or error.
		ID uint64 `json:"id"`

		// Command name. Optional.
		Command string `json:"command,omitempty"`

		// Params to be unmarshalled to body (raw payload).
		Params jsoniter.RawMessage `json:"params,omitempty"`

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

// Exchange commands with worker.
func Execute(e Endpoint, ctx Context, msg ...Message) ([]Message, error) {
	if len(msg) == 0 {
		return nil, nil
	}

	var (
		response = make([]messageFrame, 0, 5)
		result   = make([]Message, 0, 5)
		err      error
	)

	frames := make([]messageFrame, 0, len(msg))
	for _, m := range msg {
		frame, err := packFrame(m)
		if err != nil {
			return nil, err
		}

		frames = append(frames, frame)
	}

	p := payload.Payload{}

	if ctx.IsEmpty() {
		p.Context = []byte("null")
	}

	p.Context, err = jsoniter.Marshal(ctx)
	if err != nil {
		return nil, errors.E(errors.Op("encodeContext"), err)
	}

	p.Body, err = jsoniter.Marshal(frames)
	if err != nil {
		return nil, errors.E(errors.Op("encodePayload"), err)
	}

	if DebugMode {
		log.Print(color.GreenString(string(p.Body)))
	}

	out, err := e.Exec(p)
	if err != nil {
		return nil, errors.E(errors.Op("execute"), err)
	}

	if len(out.Body) == 0 {
		// worker inactive or closed
		return nil, nil
	}

	if DebugMode {
		log.Print(color.HiYellowString(string(out.Body)))
	}

	err = jsoniter.Unmarshal(out.Body, &response)
	if err != nil {
		return nil, errors.E(errors.Op("parseResponse"), err)
	}

	for _, f := range response {
		msg, err := parseFrame(f)
		if err != nil {
			return nil, err
		}

		result = append(result, msg)
	}

	return result, nil
}

func packFrame(msg Message) (messageFrame, error) {
	if msg.Command == nil {
		return messageFrame{
			ID:     msg.ID,
			Result: msg.Result,
			Error:  msg.Error,
		}, nil
	}

	name, err := commandName(msg.Command)
	if err != nil {
		return messageFrame{}, err
	}

	body, err := jsoniter.Marshal(msg.Command)
	if err != nil {
		return messageFrame{}, err
	}

	return messageFrame{
		ID:      msg.ID,
		Command: name,
		Params:  body,
		Result:  msg.Result,
		Error:   msg.Error,
	}, nil
}

func parseFrame(frame messageFrame) (Message, error) {
	if frame.Command == "" {
		return Message{
			ID:     frame.ID,
			Result: frame.Result,
			Error:  frame.Error,
		}, nil
	}

	cmd, err := initCommand(frame.Command, frame.Params)
	if err != nil {
		return Message{}, err
	}

	err = json.Unmarshal(frame.Params, &cmd)
	if err != nil {
		return Message{}, err
	}

	return Message{
		ID:      frame.ID,
		Command: cmd,
		Result:  frame.Result,
		Error:   frame.Error,
	}, nil
}
