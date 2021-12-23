package codec

import (
	"github.com/temporalio/roadrunner-temporal/internal"
)

// Codec manages payload encoding and decoding while communication with underlying worker.
type Codec interface {
	// Name returns codec name.
	Name() string

	// Execute sends message to worker and waits for the response.
	Execute(ctx *internal.Context, msg ...*internal.Message) ([]*internal.Message, error)

	// FetchWorkerInfo sends message to the worker with GetWorkerInfo payload (ID=0), this should be the first call to the PHP-SDK
	FetchWorkerInfo(workerInfo *[]*internal.WorkerInfo) error
}
