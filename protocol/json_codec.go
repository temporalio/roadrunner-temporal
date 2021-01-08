package roadrunner_temporal

import (
	"encoding/json"
	jsoniter "github.com/json-iterator/go"
	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/pkg/payload"
	"github.com/spiral/roadrunner/v2/plugins/logger"
	commonpb "go.temporal.io/api/common/v1"
)

type (
	JsonCodec struct {
		debugger *debugger
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
)

// NewJsonCodec creates new Json communication codec.
func NewJsonCodec(level DebugLevel, logger logger.Logger) Codec {
	return &JsonCodec{
		debugger: &debugger{
			level:  level,
			logger: logger,
		},
	}
}

// WithLogger creates new codes instance with attached logger.
func (c *JsonCodec) WithLogger(logger logger.Logger) Codec {
	return &JsonCodec{
		debugger: &debugger{
			level:  c.debugger.level,
			logger: logger,
		},
	}
}

// WithLogger creates new codes instance with attached logger.
func (c *JsonCodec) GetName() string {
	return "json"
}

// Exchange commands with worker.
func (c *JsonCodec) Execute(e Endpoint, ctx Context, msg ...Message) ([]Message, error) {
	if len(msg) == 0 {
		return nil, nil
	}

	c.debugger.sent(ctx, msg...)

	var (
		response = make([]messageFrame, 0, 5)
		result   = make([]Message, 0, 5)
		err      error
	)

	frames := make([]messageFrame, 0, len(msg))
	for _, m := range msg {
		frame, err := packJsonFrame(m)
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

	out, err := e.Exec(p)
	if err != nil {
		return nil, errors.E(errors.Op("execute"), err)
	}

	if len(out.Body) == 0 {
		// worker inactive or closed
		return nil, nil
	}

	err = jsoniter.Unmarshal(out.Body, &response)
	if err != nil {
		return nil, errors.E(errors.Op("parseResponse"), err)
	}

	for _, f := range response {
		msg, err := parseJsonFrame(f)
		if err != nil {
			return nil, err
		}

		result = append(result, msg)
	}

	c.debugger.received(ctx, result...)

	return result, nil
}

func packJsonFrame(msg Message) (messageFrame, error) {
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

func parseJsonFrame(frame messageFrame) (Message, error) {
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
