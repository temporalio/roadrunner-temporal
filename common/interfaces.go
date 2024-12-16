package common

import (
	"context"
	"time"

	"github.com/roadrunner-server/pool/payload"
	"github.com/roadrunner-server/pool/pool"
	staticPool "github.com/roadrunner-server/pool/pool/static_pool"
	"github.com/roadrunner-server/pool/state/process"
	"github.com/roadrunner-server/pool/worker"
	"github.com/temporalio/roadrunner-temporal/v5/internal"
	"go.temporal.io/sdk/interceptor"
	"go.uber.org/zap"
)

type Interceptor interface {
	WorkerInterceptor() interceptor.WorkerInterceptor
	Name() string
}

type Pool interface {
	// Workers return a worker list associated with the pool.
	Workers() (workers []*worker.Process)
	// RemoveWorker removes worker from the pool.
	RemoveWorker(ctx context.Context) error
	// AddWorker adds worker to the pool.
	AddWorker() error
	// QueueSize can be implemented on the pool to provide the request queue information
	QueueSize() uint64
	// Reset kill all workers inside the watcher and replaces with new
	Reset(ctx context.Context) error
	// Exec payload
	Exec(ctx context.Context, p *payload.Payload, stopCh chan struct{}) (chan *staticPool.PExec, error)
}

type Codec interface {
	// Encode encodes messages and context to the payload for the worker
	Encode(ctx *internal.Context, p *payload.Payload, msg ...*internal.Message) error
	// Decode decodes payload from the worker to the proto-message
	Decode(pld *payload.Payload, msg *[]*internal.Message) error
	// DecodeWorkerInfo decode a call to get a worker info ID=0 (initial)
	DecodeWorkerInfo(p *payload.Payload, wi *[]*internal.WorkerInfo) error
}

// Informer used to get workers from a particular plugin or set of plugins
type Informer interface {
	Workers() []*process.State
}

type Configurer interface {
	// UnmarshalKey takes a single key and unmarshal it into a Struct.
	UnmarshalKey(name string, out any) error
	// Has checks if a config section exists.
	Has(name string) bool
	// GracefulTimeout represents timeout for all servers registered in the endure
	GracefulTimeout() time.Duration
	// RRVersion returns running RR version
	RRVersion() string
	// Experimental returns true if the plugin is experimental
	Experimental() bool
}

// Server creates workers for the application.
type Server interface {
	NewPool(ctx context.Context, cfg *pool.Config, env map[string]string, _ *zap.Logger) (*staticPool.Pool, error)
	NewPoolWithOptions(ctx context.Context, cfg *pool.Config, env map[string]string, _ *zap.Logger, options ...staticPool.Options) (*staticPool.Pool, error)
}
