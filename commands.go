package roadrunner_temporal

import (
	"encoding/json"
	"time"
)

type Context struct {
	TaskQueue string    `json:"TaskQueue"`
	TickTime  time.Time `json:"tickTime"`
	Replay    bool      `json:"replay"`
}

type Frame struct {
	// ID represents command when
	ID uint64 `json:"id"`

	// Command name.
	Command string `json:"command,omitempty"`

	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}
