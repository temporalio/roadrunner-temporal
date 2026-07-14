package helpers

import (
	"context"
	"net"
	"net/rpc"

	goridgeRpc "github.com/roadrunner-server/goridge/v4/pkg/rpc"
	"github.com/roadrunner-server/pool/v2/state/process"
)

// RPCAddr is the address of the RoadRunner net/rpc control plane started by the
// test configs (rpc plugin listening on 127.0.0.1:6001).
const RPCAddr = "127.0.0.1:6001"

// RPCClient dials the RoadRunner net/rpc control plane and returns a client that
// speaks the goridge codec.
func RPCClient(ctx context.Context) (*rpc.Client, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", RPCAddr)
	if err != nil {
		return nil, err
	}

	return rpc.NewClientWithCodec(goridgeRpc.NewClientCodec(conn)), nil
}

// Workers returns the temporal plugin's worker list via the informer service.
func Workers(ctx context.Context) ([]*process.State, error) {
	c, err := RPCClient(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Close() }()

	list := struct {
		// Workers is the list of workers.
		Workers []process.State `json:"workers"`
	}{}

	err = c.Call("informer.Workers", "temporal", &list)
	if err != nil {
		return nil, err
	}

	out := make([]*process.State, len(list.Workers))
	for i := range list.Workers {
		out[i] = &list.Workers[i]
	}

	return out, nil
}

// Reset resets the temporal plugin's worker pools via the resetter service.
func Reset(ctx context.Context) error {
	c, err := RPCClient(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()

	var ret bool
	return c.Call("resetter.Reset", "temporal", &ret)
}
