package helpers

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	informerV1 "github.com/roadrunner-server/api-go/v6/informer/v1"
	"github.com/roadrunner-server/api-go/v6/informer/v1/informerV1connect"
	resetterV1 "github.com/roadrunner-server/api-go/v6/resetter/v1"
	"github.com/roadrunner-server/api-go/v6/resetter/v1/resetterV1connect"
	"github.com/roadrunner-server/api-go/v6/temporal/v1/temporalV1connect"
)

// RPCAddr is the base URL of the RoadRunner Connect-RPC plane started by the
// test configs (rpc plugin listening on 127.0.0.1:6001).
const RPCAddr = "http://127.0.0.1:6001"

// TemporalClient returns a TemporalService client bound to the test rpc plane.
func TemporalClient() temporalV1connect.TemporalServiceClient {
	return temporalV1connect.NewTemporalServiceClient(http.DefaultClient, RPCAddr)
}

// Workers returns the temporal plugin's worker list via the informer service.
func Workers(ctx context.Context) ([]*informerV1.ProcessState, error) {
	c := informerV1connect.NewInformerServiceClient(http.DefaultClient, RPCAddr)

	resp, err := c.GetWorkers(ctx, connect.NewRequest(&informerV1.GetWorkersRequest{Plugin: "temporal"}))
	if err != nil {
		return nil, err
	}

	return resp.Msg.GetWorkers(), nil
}

// Reset resets the temporal plugin's worker pools via the resetter service.
func Reset(ctx context.Context) error {
	c := resetterV1connect.NewResetterServiceClient(http.DefaultClient, RPCAddr)

	_, err := c.Reset(ctx, connect.NewRequest(&resetterV1.ResetRequest{Plugin: "temporal"}))
	return err
}
