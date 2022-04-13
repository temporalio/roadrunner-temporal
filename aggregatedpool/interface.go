package aggregatedpool

import (
	"github.com/roadrunner-server/api/v2/payload"
	"github.com/temporalio/roadrunner-temporal/internal"
)

type Codec interface {
	// Encode encodes messages and context to the payload for the worker
	Encode(ctx *internal.Context, p *payload.Payload, msg ...*internal.Message) error
	// Decode decodes payload from the worker to the proto-message
	Decode(pld *payload.Payload, msg *[]*internal.Message) error
	// DecodeWorkerInfo decode a call to get a worker info ID=0 (initial)
	DecodeWorkerInfo(p *payload.Payload, wi *[]*internal.WorkerInfo) error
}
