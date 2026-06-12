package tests

import (
	"context"
	"path"
	"sync"
	"testing"
	"tests/helpers"
	"time"

	"connectrpc.com/connect"
	protoApi "github.com/roadrunner-server/api-go/v6/temporal/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/client"
)

func Test_RPC_Methods(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-proto.yaml")

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

	t.Run("downloadWFHistory", downloadWFHistory(w.GetID(), w.GetRunID(), "HistoryLengthWorkflow", tmp))
	t.Run("replayFromJSON", replayFromJSON(tmp, "HistoryLengthWorkflow"))

	stopCh <- struct{}{}
	wg.Wait()
	time.Sleep(time.Second)
}

func downloadWFHistory(wid, rid, wname, path string) func(t *testing.T) {
	return func(t *testing.T) {
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

		resp, err := helpers.TemporalClient().DownloadWorkflowHistory(t.Context(), connect.NewRequest(req))
		require.NoError(t, err)
		require.Zero(t, resp.Msg.GetStatus().GetCode())
	}
}

func replayFromJSON(path, wname string) func(t *testing.T) {
	return func(t *testing.T) {
		req := &protoApi.ReplayRequest{
			SavePath: path,
			WorkflowType: &common.WorkflowType{
				Name: wname,
			},
		}

		resp, err := helpers.TemporalClient().ReplayFromJSON(t.Context(), connect.NewRequest(req))
		require.NoError(t, err)
		require.Zero(t, resp.Msg.GetStatus().GetCode())
	}
}
