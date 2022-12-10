package common

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
	hdr := ctx.Value(HeaderContextKey)
	if hdr == nil {
		return nil
	}

	if val, ok := hdr.(map[string]*commonpb.Payload); ok {
		return &commonpb.Header{
			Fields: val,
		}
	}

	return nil
}
