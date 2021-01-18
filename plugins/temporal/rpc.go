package temporal

import (
	"context"
)

type EmptyStruct struct{}

/*
- the method's type is exported.
- the method is exported.
- the method has two arguments, both exported (or builtin) types.
- the method's second argument is a pointer.
- the method has return type error.
*/
type rpc struct {
	srv *Plugin
}

// todo: rewrite
type RecordHeartbeatRequest struct {
	TaskToken []byte
	Details   interface{}
}

// RecordActivityHeartbeat records heartbeat for an activity.
// taskToken - is the value of the binary "TaskToken" field of the "ActivityInfo" struct retrieved inside the activity.
// details - is the progress you want to record along with heart beat for this activity.
// The errors it can return:
//	- EntityNotExistsError
//	- InternalServiceError
func (r *rpc) RecordActivityHeartbeat(in RecordHeartbeatRequest, out *bool) error {
	ctx := context.Background()

	// todo: use proper data converter type
	err := r.srv.client.RecordActivityHeartbeat(ctx, in.TaskToken, in.Details)
	if err != nil {
		return err
	}

	*out = true

	return nil
}
