package tests

import (
	"context"
	"net"
	"net/rpc"
	"path"
	"sync"
	"testing"
	"time"

	protoApi "github.com/roadrunner-server/api/v4/build/temporal/v1"
	goridgeRpc "github.com/roadrunner-server/goridge/v3/pkg/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/client"
)

const (
	download string = "temporal.DownloadWorkflowHistory"
	replay   string = "temporal.ReplayFromJSON"
)

func Test_RPC_Methods(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, "configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"HistoryLengthWorkflow")
	assert.NoError(t, err)

	time.Sleep(time.Second)
	var result any

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	assert.NoError(t, w.Get(ctx, &result))

	res := []float64{3, 8, 8, 15}
	out := result.([]interface{})

	for i := 0; i < len(res); i++ {
		if res[i] != out[i].(float64) {
			t.Fail()
		}
	}

	we, err := s.Client.DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)
	assert.Equal(t, "Completed", we.WorkflowExecutionInfo.Status.String())

	time.Sleep(time.Second)
	tmp := path.Join(t.TempDir(), "replay.json")

	t.Run("downloadWFHistory", downloadWFHistory("127.0.0.1:6001", w.GetID(), w.GetRunID(), "HistoryLengthWorkflow", tmp))
	t.Run("replayFromJSON", replayFromJSON("127.0.0.1:6001", tmp, "HistoryLengthWorkflow"))

	stopCh <- struct{}{}
	wg.Wait()
	time.Sleep(time.Second)
}

func downloadWFHistory(address, wid, rid, wname, path string) func(t *testing.T) {
	return func(t *testing.T) {
		conn, err := net.Dial("tcp", address)
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

func replayFromJSON(address, path, wname string) func(t *testing.T) {
	return func(t *testing.T) {
		conn, err := net.Dial("tcp", address)
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
