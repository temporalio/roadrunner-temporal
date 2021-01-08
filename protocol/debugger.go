package roadrunner_temporal

import (
	"github.com/fatih/color"
	jsoniter "github.com/json-iterator/go"
	"github.com/spiral/roadrunner/v2/plugins/logger"
)

// Base on JSON frames for easier debugging and testing.
type debugger struct {
	// level enables verbose logging or all incoming and outcoming messages.
	level DebugLevel

	// logger renders messages when debug enabled.
	logger logger.Logger
}

func (c *debugger) sent(ctx Context, msg ...Message) {
	if c.level <= DebugNone {
		return
	}

	frames := make([]jsonFrame, 0, len(msg))
	for _, m := range msg {
		frame, err := packJsonFrame(m)
		if err != nil {
			panic(err)
		}

		frames = append(frames, frame)
	}

	packed, err := jsoniter.Marshal(frames)
	if err != nil {
		return
	}

	logMessage := string(packed)
	if c.level >= DebugHumanized {
		logMessage = color.GreenString(string(packed))
	}

	c.logger.Debug(logMessage, "sent", true, "taskQueue", "tickTime", ctx.TickTime, ctx.TaskQueue, "replay", true)
}

func (c *debugger) received(ctx Context, msg ...Message) {
	if c.level <= DebugNone {
		return
	}

	frames := make([]jsonFrame, 0, len(msg))
	for _, m := range msg {
		frame, err := packJsonFrame(m)
		if err != nil {
			panic(err)
		}

		frames = append(frames, frame)
	}

	packed, err := jsoniter.Marshal(frames)
	if err != nil {
		return
	}

	logMessage := string(packed)

	if c.level >= DebugHumanized {
		logMessage = color.HiYellowString(string(packed))
	}

	c.logger.Debug(logMessage, "receive", true)
}
