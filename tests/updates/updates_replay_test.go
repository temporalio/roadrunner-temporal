package updates

import (
	"context"
	"net"
	"net/rpc"
	"path"
	"sync"
	"testing"
	"tests/helpers"
	"time"

	protoApi "github.com/roadrunner-server/api/v4/build/temporal/v1"
	goridgeRpc "github.com/roadrunner-server/goridge/v3/pkg/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
)

const (
	download string = "temporal.DownloadWorkflowHistory"
	replay   string = "temporal.ReplayFromJSON"
)

func TestUpdatesReplay(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

	w, err := s.Client.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		updateGreetWF)

	require.NoError(t, err)
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	handle, err := s.Client.UpdateWorkflow(ctx, w.GetID(), w.GetRunID(), addNameM, "John Doe")
	require.NoError(t, err)

	var result any

	err = handle.Get(context.Background(), &result)
	require.NoError(t, err)
	require.Equal(t, "Hello, John Doe!", result.(string))

	err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), exitSig, nil)
	require.NoError(t, err)

	time.Sleep(time.Second)

	s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED
	})

	we, err := s.Client.DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)
	assert.Equal(t, "Completed", we.WorkflowExecutionInfo.Status.String())

	time.Sleep(time.Second)
	tmp := path.Join(t.TempDir(), "replay.json")

	t.Run("downloadWFHistory", downloadWFHistory("127.0.0.1:6001", w.GetID(), w.GetRunID(), updateGreetWF, tmp))
	t.Run("replayFromJSON", replayFromJSON("127.0.0.1:6001", tmp, updateGreetWF))

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
