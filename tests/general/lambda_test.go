package tests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"tests/helpers"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runtimeAPIStub struct {
	server       *httptest.Server
	invocations  int
	window       time.Duration
	served       atomic.Int32
	acknowledged chan string
	exhausted    chan struct{}
}

func newRuntimeAPIStub(invocations int, window time.Duration) *runtimeAPIStub {
	stub := &runtimeAPIStub{
		invocations:  invocations,
		window:       window,
		acknowledged: make(chan string, invocations),
		exhausted:    make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/2018-06-01/runtime/invocation/next", func(w http.ResponseWriter, _ *http.Request) {
		served := stub.served.Add(1)
		if int(served) > stub.invocations {
			// Lambda holds the connection open until an invocation arrives.
			<-stub.exhausted
			return
		}

		w.Header().Set("Lambda-Runtime-Aws-Request-Id", "request-"+strconv.Itoa(int(served)-1))
		w.Header().Set(
			"Lambda-Runtime-Deadline-Ms",
			strconv.FormatInt(time.Now().Add(stub.window).UnixMilli(), 10),
		)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/2018-06-01/runtime/invocation/", func(w http.ResponseWriter, r *http.Request) {
		stub.acknowledged <- r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	})

	stub.server = httptest.NewServer(mux)

	return stub
}

func (s *runtimeAPIStub) host() string {
	return s.server.Listener.Addr().String()
}

func (s *runtimeAPIStub) close() {
	close(s.exhausted)
	s.server.Close()
}

// The PHP pools must outlive every invocation: only the Temporal workers are
// cycled, so the worker PIDs stay the same across invocations.
func Test_LambdaKeepsThePhpPoolAcrossInvocations(t *testing.T) {
	stub := newRuntimeAPIStub(2, time.Second*6)
	defer stub.close()

	bootLog := filepath.Join(t.TempDir(), "boots")
	t.Setenv("AWS_LAMBDA_RUNTIME_API", stub.host())
	t.Setenv("LAMBDA_BOOT_LOG", bootLog)

	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	_ = helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-lambda.yaml")

	first := waitForAcknowledgement(t, stub)
	bootsAfterFirst := phpBoots(t, bootLog)

	second := waitForAcknowledgement(t, stub)
	bootsAfterSecond := phpBoots(t, bootLog)

	assert.Equal(t, "/2018-06-01/runtime/invocation/request-0/response", first)
	assert.Equal(t, "/2018-06-01/runtime/invocation/request-1/response", second)

	require.NotEmpty(t, bootsAfterFirst, "no PHP worker was started at all")
	assert.Equal(
		t,
		bootsAfterFirst,
		bootsAfterSecond,
		"a PHP worker was started again for the second invocation: the pool did not survive",
	)

	stopCh <- struct{}{}
	wg.Wait()
}

func waitForAcknowledgement(t *testing.T, stub *runtimeAPIStub) string {
	t.Helper()

	select {
	case path := <-stub.acknowledged:
		return path
	case <-time.After(time.Second * 30):
		t.Fatal("the runtime did not acknowledge an invocation")
		return ""
	}
}

// phpBoots returns the pid of every PHP worker process the pools have started
// so far, one line per boot.
func phpBoots(t *testing.T, bootLog string) []string {
	t.Helper()

	recorded, err := os.ReadFile(bootLog)
	require.NoError(t, err)

	return strings.Fields(string(recorded))
}
