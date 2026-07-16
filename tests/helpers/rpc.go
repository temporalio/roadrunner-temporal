package helpers

import (
	"context"
	"net"
	"net/rpc"

	informerV1 "github.com/roadrunner-server/api-go/v6/informer/v1"
	resetterV1 "github.com/roadrunner-server/api-go/v6/resetter/v1"
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

	list := &informerV1.WorkersList{}
	err = c.Call("informer.GetWorkers", &informerV1.GetWorkersRequest{Plugin: "temporal"}, list)
	if err != nil {
		return nil, err
	}

	out := make([]*process.State, 0, len(list.GetWorkers()))
	for _, w := range list.GetWorkers() {
		out = append(out, &process.State{
			Pid:         int64(w.GetPid()),
			Status:      w.GetStatus(),
			NumExecs:    w.GetNumExecs(),
			Created:     w.GetCreated(),
			MemoryUsage: w.GetMemoryUsage(),
			CPUPercent:  float64(w.GetCpuPercent()),
			Command:     w.GetCommand(),
			StatusStr:   w.GetStatusStr(),
		})
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

	return c.Call("resetter.Reset", &resetterV1.ResetRequest{Plugin: "temporal"}, &resetterV1.Response{})
}
