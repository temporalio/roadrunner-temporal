package tests

import (
	"sync"
	"testing"
	"time"

	"tests/helpers"
)

func TestTemporalCheckStatus(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	_ = helpers.NewTestServer(t, stopCh, wg, "../configs/.rr-status.yaml")

	time.Sleep(time.Second)

	assertStatusOK(t, "http://127.0.0.1:35544/health?plugin=temporal")
	assertStatusOK(t, "http://127.0.0.1:35544/ready?plugin=temporal")

	stopCh <- struct{}{}

	wg.Wait()
}
