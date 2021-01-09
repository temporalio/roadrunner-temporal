package roadrunner_temporal

import (
	"github.com/fatih/color"
	jsoniter "github.com/json-iterator/go"
	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/pkg/payload"
	"github.com/spiral/roadrunner/v2/plugins/logger"
	commonpb "go.temporal.io/api/common/v1"
)

type (
	// JsonCodec can be used for debugging and log capturing reasons.
	JsonCodec struct {
		// level enables verbose logging or all incoming and outcoming messages.
		level DebugLevel

		// logger renders messages when debug enabled.
		logger logger.Logger
	}

	// jsonFrame contains message command in binary form.
	jsonFrame struct {
		// ID contains ID of the command, response or error.
		ID uint64 `json:"id"`

		// Command name. Optional.
		Command string `json:"command,omitempty"`

		// Options to be unmarshalled to body (raw payload).
		Options jsoniter.RawMessage `json:"options,omitempty"`

		// Error associated with command id.
		Error *Error `json:"error,omitempty"`

		// Payloads specific to the command or result.
		Payloads *commonpb.Payloads `json:"payloads,omitempty"`
	}
)

// NewJsonCodec creates new Json communication codec.
func NewJsonCodec(level DebugLevel, logger logger.Logger) Codec {
	return &JsonCodec{
		level:  level,
		logger: logger,
	}
}

// WithLogger creates new codes instance with attached logger.
func (c *JsonCodec) WithLogger(logger logger.Logger) Codec {
	return &JsonCodec{
		level:  c.level,
		logger: logger,
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

	var (
		response = make([]jsonFrame, 0, 5)
		result   = make([]Message, 0, 5)
		err      error
	)

	frames := make([]jsonFrame, 0, len(msg))
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

	p.Context, err = jsoniter.Marshal(ctx)
	if err != nil {
		return nil, errors.E(errors.Op("encodeContext"), err)
	}

	p.Body, err = jsoniter.Marshal(frames)
	if err != nil {
		return nil, errors.E(errors.Op("encodePayload"), err)
	}

	if c.level >= DebugNormal {
		logMessage := string(p.Body) + " " + string(p.Context)
		if c.level >= DebugHumanized {
			logMessage = color.GreenString(logMessage)
		}

		c.logger.Debug(logMessage)
	}

	out, err := e.Exec(p)
	if err != nil {
		return nil, errors.E(errors.Op("execute"), err)
	}

	if len(out.Body) == 0 {
		// worker inactive or closed
		return nil, nil
	}

	if c.level >= DebugNormal {
		logMessage := string(out.Body)
		if c.level >= DebugHumanized {
			logMessage = color.HiYellowString(logMessage)
		}

		c.logger.Debug(logMessage, "receive", true)
	}

	err = jsoniter.Unmarshal(out.Body, &response)
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

	return result, nil
}

func (c *JsonCodec) packFrame(msg Message) (jsonFrame, error) {
	if msg.Command == nil {
		return jsonFrame{
			ID:       msg.ID,
			Error:    msg.Error,
			Payloads: msg.Payloads,
		}, nil
	}

	name, err := commandName(msg.Command)
	if err != nil {
		return jsonFrame{}, err
	}

	body, err := jsoniter.Marshal(msg.Command)
	if err != nil {
		return jsonFrame{}, err
	}

	return jsonFrame{
		ID:       msg.ID,
		Command:  name,
		Options:  body,
		Error:    msg.Error,
		Payloads: msg.Payloads,
	}, nil
}

func (c *JsonCodec) parseFrame(frame jsonFrame) (Message, error) {
	if frame.Command == "" {
		return Message{
			ID:       frame.ID,
			Error:    frame.Error,
			Payloads: frame.Payloads,
		}, nil
	}

	cmd, err := initCommand(frame.Command, frame.Options)
	if err != nil {
		return Message{}, err
	}

	err = jsoniter.Unmarshal(frame.Options, &cmd)
	if err != nil {
		return Message{}, err
	}

	return Message{
		ID:       frame.ID,
		Command:  cmd,
		Error:    frame.Error,
		Payloads: frame.Payloads,
	}, nil
}
