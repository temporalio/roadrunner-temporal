package proto

import (
	"sync"

	"github.com/goccy/go-json"
	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v3/payload"
	"github.com/temporalio/roadrunner-temporal/v2/internal"
	protocolV1 "github.com/temporalio/roadrunner-temporal/v2/proto/protocol/v1"
	"go.temporal.io/sdk/converter"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// Codec uses protobuf to exchange messages with underlying workers.
type Codec struct {
	log    *zap.Logger
	dc     converter.DataConverter
	frPool sync.Pool
}

// NewCodec creates new Proto communication Codec.
func NewCodec(log *zap.Logger, dc converter.DataConverter) *Codec {
	return &Codec{
		log: log,
		dc:  dc,
		frPool: sync.Pool{
			New: func() any {
				return &protocolV1.Frame{}
			},
		},
	}
}

func (c *Codec) Encode(ctx *internal.Context, p *payload.Payload, msg ...*internal.Message) error {
	if len(msg) == 0 {
		c.log.Debug("nil message")
		return nil
	}

	request := c.getFrame()
	defer c.putFrame(request)

	request.Messages = make([]*protocolV1.Message, len(msg))

	for i := 0; i < len(msg); i++ {
		frame, err := c.packMessage(msg[i])
		if err != nil {
			return err
		}
		c.log.Debug("outgoing message", zap.Uint64("id", frame.Id), zap.ByteString("data", p.Body), zap.ByteString("context", p.Context))
		request.Messages[i] = frame
	}

	// context is always in json format
	if ctx.IsEmpty() {
		p.Context = []byte("null")
	}

	var err error
	p.Context, err = json.Marshal(ctx)
	if err != nil {
		return errors.E(errors.Op("encode_context"), err)
	}

	p.Body, err = proto.Marshal(request)
	if err != nil {
		return errors.E(errors.Op("encode_payload"), err)
	}

	return nil
}

func (c *Codec) Decode(pld *payload.Payload, result *[]*internal.Message) error {
	if len(pld.Body) == 0 || result == nil {
		// worker inactive or closed
		return nil
	}

	response := c.getFrame()
	defer c.putFrame(response)

	err := proto.Unmarshal(pld.Body, response)
	if err != nil {
		return errors.E(errors.Op("codec_parse_response"), err)
	}

	for _, f := range response.Messages {
		msg, errM := c.parseMessage(f)

		c.log.Debug("received message", zap.Any("command", msg.Command), zap.Uint64("id", msg.ID), zap.ByteString("data", pld.Body))
		if errM != nil {
			return errM
		}

		*result = append(*result, msg)
	}

	return nil
}

// DecodeWorkerInfo ... info []*internal.Message is read-only
// wi *[]*internal.WorkerInfo should be pre-allocated
func (c *Codec) DecodeWorkerInfo(p *payload.Payload, wi *[]*internal.WorkerInfo) error {
	const op = errors.Op("workflow_fetch_wf_info")

	// should be only 1
	info := make([]*internal.Message, 0, 1)
	err := c.Decode(p, &info)
	if err != nil {
		return err
	}

	if len(info) != 1 {
		return errors.E(op, errors.Str("unable to read worker info"))
	}

	// internal convention
	if info[0].ID != 0 {
		return errors.E(op, errors.Errorf("fetch confirmation missing, need ID: 0, got: %d", info[0].ID))
	}

	payloads := info[0].Payloads.GetPayloads()

	for i := 0; i < len(payloads); i++ {
		tmp := &internal.WorkerInfo{}
		err = c.dc.FromPayload(payloads[i], tmp)
		if err != nil {
			return errors.E(op, err)
		}
		*wi = append(*wi, tmp)
	}

	return nil
}

func (c *Codec) packMessage(msg *internal.Message) (*protocolV1.Message, error) {
	var err error

	frame := &protocolV1.Message{
		Id:       msg.ID,
		Payloads: msg.Payloads,
		Failure:  msg.Failure,
		Header:   msg.Header,
	}

	if msg.Command != nil {
		frame.Command, err = internal.CommandName(msg.Command)
		if err != nil {
			return nil, err
		}

		frame.Options, err = json.Marshal(msg.Command)
		if err != nil {
			return nil, err
		}
	}

	return frame, nil
}

func (c *Codec) parseMessage(frame *protocolV1.Message) (*internal.Message, error) {
	const op = errors.Op("proto_codec_parse_message")
	var err error

	msg := &internal.Message{
		ID:       frame.Id,
		Payloads: frame.Payloads,
		Failure:  frame.Failure,
	}

	if frame.Command != "" {
		msg.Command, err = internal.InitCommand(frame.Command)
		if err != nil {
			return nil, errors.E(op, err)
		}

		err = json.Unmarshal(frame.Options, &msg.Command)
		if err != nil {
			return nil, errors.E(op, err)
		}
	}

	return msg, nil
}

func (c *Codec) getFrame() *protocolV1.Frame {
	return c.frPool.Get().(*protocolV1.Frame)
}

func (c *Codec) putFrame(fr *protocolV1.Frame) {
	fr.Reset()
	c.frPool.Put(fr)
}
