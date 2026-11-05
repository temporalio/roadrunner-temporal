package tests

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeRuntimeAPI is the AWS Lambda Runtime API, enough of it to drive a real
// lambda.Start loop: the runtime long-polls /next, we hand it one invocation at
// a time, and we observe when it answers.
type fakeRuntimeAPI struct {
	server   *httptest.Server
	pending  chan *invocation
	inFlight atomic.Value
	counter  atomic.Int64
}

type invocation struct {
	deadline time.Time
	answered chan struct{}
}

func startFakeRuntimeAPI(t *testing.T) *fakeRuntimeAPI {
	t.Helper()

	api := &fakeRuntimeAPI{pending: make(chan *invocation)}

	mux := http.NewServeMux()
	mux.HandleFunc("/2018-06-01/runtime/invocation/next", api.next)
	mux.HandleFunc("/2018-06-01/runtime/invocation/", api.answer)
	mux.HandleFunc("/2018-06-01/runtime/init/error", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the runtime reported an init error")
		w.WriteHeader(http.StatusAccepted)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	api.server = &httptest.Server{Listener: listener, Config: &http.Server{Handler: mux, ReadHeaderTimeout: time.Minute}}
	// The server is deliberately never closed: aws-lambda-go treats any failure
	// of the Runtime API as fatal and kills the process. Real AWS simply never
	// answers /next again, which is what a blocked handler reproduces.
	api.server.Start()

	t.Setenv("AWS_LAMBDA_RUNTIME_API", strings.TrimPrefix(api.server.URL, "http://"))

	return api
}

func (api *fakeRuntimeAPI) next(w http.ResponseWriter, r *http.Request) {
	var current *invocation

	select {
	case current = <-api.pending:
	case <-r.Context().Done():
		return
	}

	api.inFlight.Store(current)

	w.Header().Set("Lambda-Runtime-Aws-Request-Id", fmt.Sprintf("req-%d", api.counter.Add(1)))
	w.Header().Set("Lambda-Runtime-Deadline-Ms", fmt.Sprintf("%d", current.deadline.UnixMilli()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
}

func (api *fakeRuntimeAPI) answer(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusAccepted)

	if current, ok := api.inFlight.Load().(*invocation); ok && current != nil {
		api.inFlight.Store((*invocation)(nil))
		close(current.answered)
	}
}

// invoke hands one invocation to the runtime and returns a channel that closes
// when the runtime answers it.
func (api *fakeRuntimeAPI) invoke(t *testing.T, budget time.Duration) <-chan struct{} {
	t.Helper()

	current := &invocation{deadline: time.Now().Add(budget), answered: make(chan struct{})}

	select {
	case api.pending <- current:
	case <-time.After(time.Second * 10):
		t.Fatal("the runtime never polled for the next invocation")
	}

	return current.answered
}

func awaitAnswer(t *testing.T, answered <-chan struct{}, within time.Duration) {
	t.Helper()

	select {
	case <-answered:
	case <-time.After(within):
		t.Fatalf("the invocation was not answered within %s", within)
	}
}
