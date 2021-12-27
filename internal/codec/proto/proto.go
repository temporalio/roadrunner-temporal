package proto

import (
	"fmt"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/spiral/roadrunner/v2/utils"
	"github.com/spiral/sdk-go/workflow"
	"go.uber.org/zap"

	jsoniter "github.com/json-iterator/go"
	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/payload"
	"github.com/spiral/roadrunner/v2/pool"
	temporalClient "github.com/spiral/sdk-go/client"
	"github.com/spiral/sdk-go/converter"
	"github.com/spiral/sdk-go/internalbindings"
	"github.com/spiral/sdk-go/worker"
	"github.com/temporalio/roadrunner-temporal/internal"
	_codec "github.com/temporalio/roadrunner-temporal/internal/codec"
	protocolV1 "github.com/temporalio/roadrunner-temporal/proto/protocol/v1"
	"google.golang.org/protobuf/proto"
)

const name string = "protobuf"

// ProtoCodec uses protobuf to exchange messages with underlying workers.
type codec struct {
	sync.RWMutex
	log *zap.Logger

	dc converter.DataConverter

	rrPool pool.Pool
	frPool sync.Pool
}

// NewCodec creates new Proto communication codec.
func NewCodec(pool pool.Pool, log *zap.Logger, dc converter.DataConverter) _codec.Codec {
	return &codec{
		rrPool: pool,
		log:    log,
		dc:     dc,
		frPool: sync.Pool{
			New: func() interface{} {
				return &protocolV1.Frame{}
			},
		},
	}
}

// Name returns codec name.
func (c *codec) Name() string {
	return name
}

// Execute exchanges commands with worker.
func (c *codec) Execute(ctx *internal.Context, msg ...*internal.Message) ([]*internal.Message, error) {
	if len(msg) == 0 {
		c.log.Debug("nil message")
		return nil, nil
	}

	request := c.getFrame()
	response := c.getFrame()
	defer c.putFrame(response)
	defer c.putFrame(request)

	var result = make([]*internal.Message, 0, 5)
	request.Messages = make([]*protocolV1.Message, len(msg))

	for i := 0; i < len(msg); i++ {
		frame, err := c.packMessage(msg[i])
		if err != nil {
			return nil, err
		}
		request.Messages[i] = frame
	}

	p := &payload.Payload{}

	// context is always in json format
	if ctx.IsEmpty() {
		p.Context = []byte("null")
	}

	var err error
	p.Context, err = jsoniter.Marshal(ctx)
	if err != nil {
		return nil, errors.E(errors.Op("encode_context"), err)
	}

	p.Body, err = proto.Marshal(request)
	if err != nil {
		return nil, errors.E(errors.Op("encode_payload"), err)
	}

	c.log.Debug("outgoing message", zap.String("data", color.GreenString(utils.AsString(p.Body)+" "+utils.AsString(p.Context))))

	c.RLock()
	out, err := c.rrPool.Exec(p)
	c.RUnlock()
	if err != nil {
		return nil, errors.E(errors.Op("codec_execute"), err)
	}

	if len(out.Body) == 0 {
		// worker inactive or closed
		return nil, nil
	}

	err = proto.Unmarshal(out.Body, response)
	if err != nil {
		return nil, errors.E(errors.Op("codec_parse_response"), err)
	}

	c.log.Debug("received message", zap.String("data", color.HiYellowString(utils.AsString(out.Body))))

	for _, f := range response.Messages {
		msg, errM := c.parseMessage(f)
		if errM != nil {
			return nil, errM
		}

		result = append(result, msg)
	}

	return result, nil
}

func (c *codec) FetchWorkerInfo(workerInfo *[]*internal.WorkerInfo) error {
	const op = errors.Op("workflow_fetch_wf_info")

	// fetch wf info
	info, err := c.Execute(&internal.Context{}, &internal.Message{ID: 0, Command: internal.GetWorkerInfo{}})
	if err != nil {
		return errors.E(op, err)
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
		wi := &internal.WorkerInfo{}
		err = c.dc.FromPayload(payloads[i], wi)
		if err != nil {
			return errors.E(op, err)
		}
		*workerInfo = append(*workerInfo, wi)
	}

	return nil
}

func (c *codec) FetchWFInfo(client temporalClient.Client, definition internalbindings.WorkflowDefinition) (map[string]internal.WorkflowInfo, []worker.Worker, error) {
	const op = errors.Op("workflow_fetch_wf_info")

	// fetch wf info
	info, err := c.Execute(&internal.Context{}, &internal.Message{ID: 0, Command: internal.GetWorkerInfo{}})
	if err != nil {
		return nil, nil, errors.E(op, err)
	}

	if len(info) != 1 {
		return nil, nil, errors.E(op, errors.Str("unable to read worker info"))
	}

	// internal convention
	if info[0].ID != 0 {
		return nil, nil, errors.E(op, errors.Errorf("fetch confirmation missing, need ID: 0, got: %d", info[0].ID))
	}

	workerInfo := make([]*internal.WorkerInfo, 0, 5)
	payloads := info[0].Payloads.GetPayloads()

	workflows := make(map[string]internal.WorkflowInfo)
	workers := make([]worker.Worker, 0, 1)

	for i := 0; i < len(payloads); i++ {
		wi := &internal.WorkerInfo{}
		err = c.dc.FromPayload(payloads[i], wi)
		if err != nil {
			return nil, nil, errors.E(op, err)
		}
		workerInfo = append(workerInfo, wi)
	}

	for i := 0; i < len(workerInfo); i++ {
		c.log.Debug("worker info", zap.String("taskqueue", workerInfo[i].TaskQueue), zap.Any("options", workerInfo[i].Options))

		// todo(rustatian) properly set grace timeout
		workerInfo[i].Options.WorkerStopTimeout = time.Second * 30

		if workerInfo[i].TaskQueue == "" {
			workerInfo[i].TaskQueue = temporalClient.DefaultNamespace
		}

		if workerInfo[i].Options.Identity == "" {
			workerInfo[i].Options.Identity = fmt.Sprintf(
				"%s:%s",
				workerInfo[i].TaskQueue,
				uuid.NewString(),
			)
		}

		wrk := worker.New(client, workerInfo[i].TaskQueue, workerInfo[i].Options)

		for j := 0; j < len(workerInfo[i].Workflows); j++ {
			c.log.Debug("register workflow with options", zap.String("taskqueue", workerInfo[i].TaskQueue), zap.String("workflow name", workerInfo[i].Workflows[j].Name))

			wrk.RegisterWorkflowWithOptions(definition, workflow.RegisterOptions{
				Name:                          workerInfo[i].Workflows[j].Name,
				DisableAlreadyRegisteredCheck: false,
			})

			workflows[workerInfo[i].Workflows[j].Name] = workerInfo[i].Workflows[j]
		}

		workers = append(workers, wrk)
	}

	return workflows, workers, nil
}

func (c *codec) packMessage(msg *internal.Message) (*protocolV1.Message, error) {
	var err error

	frame := &protocolV1.Message{
		Id:       msg.ID,
		Payloads: msg.Payloads,
		Failure:  msg.Failure,
	}

	if msg.Command != nil {
		frame.Command, err = internal.CommandName(msg.Command)
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

func (c *codec) parseMessage(frame *protocolV1.Message) (*internal.Message, error) {
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

		err = jsoniter.Unmarshal(frame.Options, &msg.Command)
		if err != nil {
			return nil, errors.E(op, err)
		}
	}

	return msg, nil
}

func (c *codec) getFrame() *protocolV1.Frame {
	return c.frPool.Get().(*protocolV1.Frame)
}

func (c *codec) putFrame(fr *protocolV1.Frame) {
	fr.Reset()
	c.frPool.Put(fr)
}
