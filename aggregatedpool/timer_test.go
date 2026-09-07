package aggregatedpool

import (
	"errors"
	"sync"
	"testing"
	"time"

	"log/slog"

	"github.com/roadrunner-server/pool/v2/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/temporalio/roadrunner-temporal/v6/canceller"
	"github.com/temporalio/roadrunner-temporal/v6/internal"
	"github.com/temporalio/roadrunner-temporal/v6/queue"
	"github.com/temporalio/roadrunner-temporal/v6/registry"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/workflow"
)

// timerEnv reproduces the sdk-go NewTimer contract: a non-positive duration is
// resolved inside the call (callback invoked inline, nil TimerID returned).
// Everything else is only what getContext/handleMessage touch.
type timerEnv struct {
	bindings.WorkflowEnvironment
	info workflow.Info
}

func (e *timerEnv) WorkflowInfo() *workflow.Info { return &e.info }
func (e *timerEnv) Now() time.Time               { return time.Unix(0, 0) }
func (e *timerEnv) IsReplaying() bool            { return false }

func (e *timerEnv) NewTimer(d time.Duration, _ workflow.TimerOptions, callback bindings.ResultHandler) *bindings.TimerID {
	if d <= 0 {
		callback(nil, nil)
		return nil
	}

	id := bindings.TimerID{}

	return &id
}

// capturingCodec captures what a flush would send to PHP and stops flushQueue
// before it reaches the pool.
type capturingCodec struct {
	err      error
	encoded  []*internal.Message
	encCalls int
}

func (c *capturingCodec) Encode(_ *internal.Context, _ *payload.Payload, msgs ...*internal.Message) error {
	c.encCalls++
	c.encoded = append(c.encoded, msgs...)
	return c.err
}

func (c *capturingCodec) Decode(_ *payload.Payload, _ *[]*internal.Message) error { return nil }

func (c *capturingCodec) DecodeWorkerInfo(_ *payload.Payload, _ *[]*internal.WorkerInfo) error {
	return nil
}

// A zero-duration timer is resolved by the SDK inside NewTimer, so its response
// is queued with no command of its own to carry a flush. Without the drain-loop
// flush nothing is ever sent and PHP hangs until the workflow task times out.
func TestDrainPipeline_ZeroDurationTimerResponseIsFlushed(t *testing.T) {
	stop := errors.New("stop before the pool")
	codec := &capturingCodec{err: stop}

	wp := &Workflow{
		log:          slog.New(slog.DiscardHandler),
		env:          &timerEnv{},
		codec:        codec,
		pool:         &recordingPool{},
		mq:           queue.NewMessageQueue(func() uint64 { return 0 }),
		canceller:    new(canceller.Canceller),
		nexusStarted: new(registry.NexusStartedRegistry),
		pldPool:      &sync.Pool{New: func() any { return new(payload.Payload) }},
		pipeline: []*internal.Message{{
			ID:      42,
			Command: &internal.NewTimer{Milliseconds: 0},
		}},
	}
	wp.inLoop = 1

	err := wp.drainPipeline()

	require.ErrorIs(t, err, stop, "the queued response must be flushed after the batch")
	require.Len(t, codec.encoded, 1)
	assert.Equal(t, uint64(42), codec.encoded[0].ID)
}

// A real timer keeps its canceller slot and queues nothing synchronously.
func TestDrainPipeline_PositiveDurationTimerQueuesNothing(t *testing.T) {
	codec := &capturingCodec{}

	wp := &Workflow{
		log:          slog.New(slog.DiscardHandler),
		env:          &timerEnv{},
		codec:        codec,
		pool:         &recordingPool{},
		mq:           queue.NewMessageQueue(func() uint64 { return 0 }),
		canceller:    new(canceller.Canceller),
		nexusStarted: new(registry.NexusStartedRegistry),
		pldPool:      &sync.Pool{New: func() any { return new(payload.Payload) }},
		pipeline: []*internal.Message{{
			ID:      43,
			Command: &internal.NewTimer{Milliseconds: 100},
		}},
	}
	wp.inLoop = 1

	require.NoError(t, wp.drainPipeline())
	assert.Zero(t, codec.encCalls, "a scheduled timer must not trigger a flush")
	assert.Empty(t, wp.mq.Messages())
}
