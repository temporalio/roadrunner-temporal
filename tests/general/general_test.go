package tests

import (
    "context"
    goridgeRpc "github.com/roadrunner-server/goridge/v3/pkg/rpc"
    "github.com/roadrunner-server/pool/state/process"
    "net"
    "net/rpc"
    "sync"
    "testing"
    "time"

    "tests/helpers"

    "github.com/stretchr/testify/assert"
    "go.temporal.io/sdk/client"
)

func Test_HistoryLen(t *testing.T) {
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
    stopCh <- struct{}{}
    wg.Wait()
    time.Sleep(time.Second)
}

func Test_DisabledActivityWorkers(t *testing.T) {
    stopCh := make(chan struct{}, 1)
    wg := &sync.WaitGroup{}
    wg.Add(1)
    s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-disable-activity-worker.yaml")

    assertWorkers(t, 1)

    w, err := s.Client.ExecuteWorkflow(
        context.Background(),
        client.StartWorkflowOptions{
            TaskQueue: "default",
        },
        "QueryWorkflow",
        "Hello World",
    )
    assert.NoError(t, err)

    err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), "add", 88)
    assert.NoError(t, err)
    time.Sleep(time.Millisecond * 500)

    v, err := s.Client.QueryWorkflow(context.Background(), w.GetID(), w.GetRunID(), "get", nil)
    assert.NoError(t, err)

    var r int
    assert.NoError(t, v.Get(&r))
    assert.Equal(t, 88, r)

    assert.NoError(t, w.Get(context.Background(), &r))
    assert.Equal(t, 88, r)
    stopCh <- struct{}{}
    wg.Wait()
}

func assertWorkers(t *testing.T, workers int) {
    conn, err := net.Dial("tcp", "127.0.0.1:6001")
    assert.NoError(t, err)
    c := rpc.NewClientWithCodec(goridgeRpc.NewClientCodec(conn))
    // WorkerList contains list of workers.
    list := struct {
        // Workers is list of workers.
        Workers []process.State `json:"workers"`
    }{}

    err = c.Call("informer.Workers", "temporal", &list)
    assert.NoError(t, err)
    assert.Len(t, list.Workers, workers)
}
