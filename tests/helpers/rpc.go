package helpers

import (
	"context"
	"io"
	"net"
	"net/rpc"
	"testing"
	"time"

	protoApi "github.com/roadrunner-server/api-go/v6/temporal/v1"
	goridgeRpc "github.com/roadrunner-server/goridge/v4/pkg/rpc"
	"github.com/roadrunner-server/pool/v2/state/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/common/v1"
)

const (
	download string = "temporal.DownloadWorkflowHistory"
	replay   string = "temporal.ReplayFromJSON"
)

func GetWorkers(t *testing.T) []process.State {
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", "127.0.0.1:6001")
	assert.NoError(t, err)
	c := rpc.NewClientWithCodec(goridgeRpc.NewClientCodec(conn))
	// WorkerList contains list of workers.
	list := struct {
		// Workers is list of workers.
		Workers []process.State `json:"workers"`
	}{}

	err = c.Call("informer.Workers", "temporal", &list)
	assert.NoError(t, err)
	assert.Len(t, list.Workers, 5)

	return list.Workers
}

func ResetWorkers(t *testing.T) {
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", "127.0.0.1:6001")
	assert.NoError(t, err)
	c := rpc.NewClientWithCodec(goridgeRpc.NewClientCodec(conn))

	var ret bool
	err = c.Call("resetter.Reset", "temporal", &ret)
	assert.NoError(t, err)
	require.True(t, ret)
}

func GetActivities(t *testing.T) []string {
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", "127.0.0.1:6001")
	assert.NoError(t, err)
	c := rpc.NewClientWithCodec(goridgeRpc.NewClientCodec(conn))

	res := make([]string, 0, 10)

	err = c.Call("temporal.GetActivityNames", true, &res)
	assert.NoError(t, err)

	return res
}

func GetWorkflows(t *testing.T) []string {
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", "127.0.0.1:6001")
	assert.NoError(t, err)
	c := rpc.NewClientWithCodec(goridgeRpc.NewClientCodec(conn))

	res := make([]string, 0, 10)

	err = c.Call("temporal.GetWorkflowNames", true, &res)
	assert.NoError(t, err)

	return res
}

// GetStatsd returns the counters from the statsd admin port.
func GetStatsd(ctx context.Context) (string, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp4", "127.0.0.1:8126")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	_, err = conn.Write([]byte("counters"))
	if err != nil {
		return "", err
	}

	_ = conn.SetReadDeadline(time.Now().Add(time.Second * 2))
	d, _ := io.ReadAll(conn)
	return string(d), nil
}

func DownloadWFHistory(address, wid, rid, wname, path string) func(t *testing.T) {
	return func(t *testing.T) {
		conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", address)
		require.NoError(t, err)
		client := rpc.NewClientWithCodec(goridgeRpc.NewClientCodec(conn))

		req := &protoApi.ReplayRequest{
			SavePath: path,
			WorkflowType: &common.WorkflowType{
				Name: wname,
			},
			WorkflowExecution: &common.WorkflowExecution{
				WorkflowId: wid,
				RunId:      rid,
			},
		}
		resp := &protoApi.ReplayResponse{}
		err = client.Call(download, req, resp)
		require.NoError(t, err)
	}
}

func ReplayFromJSON(address, path, wname string) func(t *testing.T) {
	return func(t *testing.T) {
		conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", address)
		require.NoError(t, err)
		client := rpc.NewClientWithCodec(goridgeRpc.NewClientCodec(conn))

		req := &protoApi.ReplayRequest{
			SavePath: path,
			WorkflowType: &common.WorkflowType{
				Name: wname,
			},
		}
		resp := &protoApi.ReplayResponse{}
		err = client.Call(replay, req, resp)
		require.NoError(t, err)
	}
}
