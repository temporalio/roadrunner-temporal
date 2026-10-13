package api

import (
	"context"

	commonpb "go.temporal.io/api/common/v1"
)

type ContextKey struct {
	name string
}

func (ck *ContextKey) String() string {
	return ck.name
}

var (
	// HeaderContextKey is RR <-> Temporal context key
	HeaderContextKey = &ContextKey{name: "headers"} //nolint:gochecknoglobals
)

func ActivityHeadersFromCtx(ctx context.Context) *commonpb.Header {
	val, _ := ctx.Value(HeaderContextKey).(map[string]*commonpb.Payload)
	if len(val) == 0 {
		return nil
	}
	return &commonpb.Header{Fields: val}
}
