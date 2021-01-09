package roadrunner_temporal

import (
	"github.com/golang/protobuf/proto"
	jsoniter "github.com/json-iterator/go"
	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/pkg/payload"
	"github.com/spiral/roadrunner/v2/plugins/logger"
	"github.com/temporalio/roadrunner-temporal/protocol/internal"
)

type (
	ProtoCodec struct {
		debugger *debugger
	}
)

// NewJsonCodec creates new Json communication codec.
func NewProtoCodec(level DebugLevel, logger logger.Logger) Codec {
	return &ProtoCodec{
		debugger: &debugger{
			level:  level,
			logger: logger,
		},
	}
}

// WithLogger creates new codes instance with attached logger.
func (c *ProtoCodec) WithLogger(logger logger.Logger) Codec {
	return &ProtoCodec{
		debugger: &debugger{
			level:  c.debugger.level,
			logger: logger,
		},
	}
}

// WithLogger creates new codes instance with attached logger.
func (c *ProtoCodec) GetName() string {
	return "protobuf"
}

// Exchange commands with worker.
func (c *ProtoCodec) Execute(e Endpoint, ctx Context, msg ...Message) ([]Message, error) {
	if len(msg) == 0 {
		return nil, nil
	}

	c.debugger.sent(ctx, msg...)

	var (
		request  = &internal.Frame{}
		response = &internal.Frame{}
		result   = make([]Message, 0, 5)
		err      error
	)

	for _, m := range msg {
		frame, err := c.packMessage(m)
		if err != nil {
			return nil, err
		}

		request.Messages = append(request.Messages, frame)
	}

	p := payload.Payload{}

	// context is always in json format
	if ctx.IsEmpty() {
		p.Context = []byte("null")
	}

	p.Context, err = jsoniter.Marshal(ctx)
	if err != nil {
		return nil, errors.E(errors.Op("encodeContext"), err)
	}

	p.Body, err = proto.Marshal(request)
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

	err = proto.Unmarshal(out.Body, response)
	if err != nil {
		return nil, errors.E(errors.Op("parseResponse"), err)
	}

	for _, f := range response.Messages {
		msg, err := c.parseMessage(f)
		if err != nil {
			return nil, err
		}

		result = append(result, msg)
	}

	c.debugger.received(ctx, result...)

	return result, nil
}

func (c *ProtoCodec) packMessage(msg Message) (*internal.Message, error) {
	var err error

	frame := &internal.Message{
		Id:       msg.ID,
		Payloads: msg.Payloads,
	}

	if msg.Error != nil {
		frame.Error = &internal.Error{
			Code:    msg.Error.Code,
			Message: msg.Error.Message,
		}

		frame.Error.Data, err = jsoniter.Marshal(msg.Error.Data)
		if err != nil {
			return nil, err
		}
	}

	if msg.Command != nil {
		frame.Command, err = commandName(msg.Command)
		if err != nil {
			return nil, err
		}

		frame.Options, err = jsoniter.Marshal(msg.Command)
		if err != nil {
			return nil, err
		}
	}

	return frame, nil
}

func (c *ProtoCodec) parseMessage(frame *internal.Message) (Message, error) {
	var err error

	msg := Message{
		ID:       frame.Id,
		Payloads: frame.Payloads,
	}

	if frame.Error != nil {
		msg.Error = &Error{
			Code:    frame.Error.Code,
			Message: frame.Error.Message,
		}

		err = jsoniter.Unmarshal(frame.Error.Data, &msg.Error.Data)
		if err != nil {
			return Message{}, err
		}
	}

	if frame.Command != "" {
		msg.Command, err = initCommand(frame.Command, frame.Options)
		if err != nil {
			return Message{}, err
		}

		err = jsoniter.Unmarshal(frame.Options, &msg.Command)
		if err != nil {
			return Message{}, err
		}
	}

	return msg, nil
}
