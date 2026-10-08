package tests

import (
	"context"
	"path"
	"sync"
	"testing"
	"tests/helpers"
	"time"

	"github.com/stretchr/testify/assert"
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

	assert.Equal(t, []any{3.0, 8.0, 8.0, 15.0}, result)

	we, err := s.Client.DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)
	assert.Equal(t, "Completed", we.WorkflowExecutionInfo.Status.String())

	time.Sleep(time.Second)
	tmp := path.Join(t.TempDir(), "replay.json")

	t.Run("downloadWFHistory", helpers.DownloadWFHistory("127.0.0.1:6001", w.GetID(), w.GetRunID(), "HistoryLengthWorkflow", tmp))
	t.Run("replayFromJSON", helpers.ReplayFromJSON("127.0.0.1:6001", tmp, "HistoryLengthWorkflow"))

	stopCh <- struct{}{}
	wg.Wait()
	time.Sleep(time.Second)
}
