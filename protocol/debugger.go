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

func (dbg *debugger) sent(ctx Context, msg ...Message) {
	if dbg.level <= DebugNone {
		return
	}

	frames := make([]jsonFrame, 0, len(msg))
	for _, m := range msg {
		frame, err := dbg.packFrame(m)
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
	if dbg.level >= DebugHumanized {
		logMessage = color.GreenString(string(packed))
	}

	dbg.logger.Debug(
		logMessage,
		"sent",
		true,
		"taskQueue",
		ctx.TaskQueue,
		"tickTime",
		ctx.TickTime,
		"replay",
		ctx.Replay,
	)
}

func (dbg *debugger) received(ctx Context, msg ...Message) {
	if dbg.level <= DebugNone {
		return
	}

	frames := make([]jsonFrame, 0, len(msg))
	for _, m := range msg {
		frame, err := dbg.packFrame(m)
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

	if dbg.level >= DebugHumanized {
		logMessage = color.HiYellowString(string(packed))
	}

	dbg.logger.Debug(logMessage, "receive", true)
}

func (dbg *debugger) receivedRaw(ctx Context, msg ...Message) {
	if dbg.level <= DebugNone {
		return
	}

	frames := make([]jsonFrame, 0, len(msg))
	for _, m := range msg {
		frame, err := dbg.packFrame(m)
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

	if dbg.level >= DebugHumanized {
		logMessage = color.HiYellowString(string(packed))
	}

	dbg.logger.Debug(logMessage, "receive", true)
}

func (dbg *debugger) packFrame(msg Message) (jsonFrame, error) {
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
