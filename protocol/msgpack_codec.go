package roadrunner_temporal

import (
	"bytes"
	jsoniter "github.com/json-iterator/go"
	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/pkg/payload"
	"github.com/spiral/roadrunner/v2/plugins/logger"
	"github.com/vmihailenco/msgpack/v5"
	commonpb "go.temporal.io/api/common/v1"
)

type (
	MsgpackCodec struct {
		debugger *debugger
		encoder  *msgpack.Encoder
		decoder  *msgpack.Decoder
	}

	// jsonFrame contains message command in binary form.
	msgpackFrame struct {
		// ID contains ID of the command, response or error.
		ID uint64 `json:"id"`

		// Command name. Optional.
		Command string `json:"command,omitempty"`

		// Params to be unmarshalled to body (raw payload).
		Params []byte `json:"params,omitempty"`

		// Result always contains array of values.
		Result []*commonpb.Payload `json:"result,omitempty"`

		// Error associated with command id.
		Error *Error `json:"error,omitempty"`
	}
)

// NewJsonCodec creates new Json communication codec.
func NewMsgpackCodec(level DebugLevel, logger logger.Logger) Codec {

	encoder := msgpack.NewEncoder(nil)
	encoder.SetCustomStructTag("json")

	decoder := msgpack.NewDecoder(nil)
	decoder.SetCustomStructTag("json")

	return &MsgpackCodec{
		encoder: encoder,
		decoder: decoder,
		debugger: &debugger{
			level:  level,
			logger: logger,
		},
	}
}

// WithLogger creates new codes instance with attached logger.
func (c *MsgpackCodec) WithLogger(logger logger.Logger) Codec {
	return &MsgpackCodec{
		encoder: c.encoder,
		decoder: c.decoder,
		debugger: &debugger{
			level:  c.debugger.level,
			logger: logger,
		},
	}
}

// WithLogger creates new codes instance with attached logger.
func (c *MsgpackCodec) GetName() string {
	return "msgpack"
}

// Exchange commands with worker.
func (c *MsgpackCodec) Execute(e Endpoint, ctx Context, msg ...Message) ([]Message, error) {
	if len(msg) == 0 {
		return nil, nil
	}

	c.debugger.sent(ctx, msg...)

	var (
		response = make([]msgpackFrame, 0, 5)
		result   = make([]Message, 0, 5)
		err      error
	)

	frames := make([]msgpackFrame, 0, len(msg))
	for _, m := range msg {
		frame, err := c.packFrame(m)
		if err != nil {
			return nil, err
		}

		frames = append(frames, frame)
	}

	p := payload.Payload{}

	if ctx.IsEmpty() {
		p.Context = []byte("null")
	}

	// always carry context via JSON (minimal overhead)
	p.Context, err = jsoniter.Marshal(ctx)
	if err != nil {
		return nil, errors.E(errors.Op("encodeContext"), err)
	}

	p.Body, err = c.Marshal(frames)
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

	err = c.Unmarshal(out.Body, &response)
	if err != nil {
		return nil, errors.E(errors.Op("parseResponse"), err)
	}

	for _, f := range response {
		msg, err := c.parseFrame(f)
		if err != nil {
			return nil, err
		}

		result = append(result, msg)
	}

	c.debugger.received(ctx, result...)

	return result, nil
}

func (c *MsgpackCodec) packFrame(msg Message) (msgpackFrame, error) {
	if msg.Command == nil {
		return msgpackFrame{
			ID:     msg.ID,
			Result: msg.Result,
			Error:  msg.Error,
		}, nil
	}

	name, err := commandName(msg.Command)
	if err != nil {
		return msgpackFrame{}, err
	}

	body, err := c.Marshal(msg.Command)
	if err != nil {
		return msgpackFrame{}, err
	}

	return msgpackFrame{
		ID:      msg.ID,
		Command: name,
		Params:  body,
		Result:  msg.Result,
		Error:   msg.Error,
	}, nil
}

func (c *MsgpackCodec) parseFrame(frame msgpackFrame) (Message, error) {
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

	err = c.Unmarshal(frame.Params, &cmd)
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

func (c *MsgpackCodec) Marshal(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	c.encoder.Reset(&buf)

	err := c.encoder.Encode(v)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), err
}

func (c *MsgpackCodec) Unmarshal(data []byte, v interface{}) error {
	c.decoder.Reset(bytes.NewReader(data))
	return c.decoder.Decode(v)
}
