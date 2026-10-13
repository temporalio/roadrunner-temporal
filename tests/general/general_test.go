package tests

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/rpc"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	goridgeRpc "github.com/roadrunner-server/goridge/v4/pkg/rpc"
	"github.com/roadrunner-server/pool/v2/state/process"

	"tests/helpers"

	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/client"
)

func Test_DisabledActivityWorkers(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-disable-activity-worker.yaml")

	assertWorkers(t, 1)

	time.Sleep(time.Second)

	assertStatusOK(t, "http://127.0.0.1:35545/health?plugin=temporal")
	assertStatusOK(t, "http://127.0.0.1:35545/ready?plugin=temporal")

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
	assert.Len(t, list.Workers, workers)
}

func assertStatusOK(t *testing.T, url string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, `[{"plugin_name":"temporal","error_message":"","status_code":200}]`, string(body))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
