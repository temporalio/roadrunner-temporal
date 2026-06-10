package updates

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
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
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

	handle, err := s.Client.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		RunID:        w.GetRunID(),
		WorkflowID:   w.GetID(),
		UpdateName:   addNameM,
		Args:         []any{"John Doe"},
		WaitForStage: client.WorkflowUpdateStageAccepted,
	})
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

	t.Run("downloadWFHistory", downloadWFHistory(w.GetID(), w.GetRunID(), updateGreetWF, tmp))
	t.Run("replayFromJSON", replayFromJSON(tmp, updateGreetWF))

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
