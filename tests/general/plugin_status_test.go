package tests

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/roadrunner-server/config/v5"
	"github.com/roadrunner-server/endure/v2"
	"github.com/roadrunner-server/logger/v5"
	"github.com/roadrunner-server/server/v5"
	"github.com/roadrunner-server/status/v5"
	"github.com/stretchr/testify/require"
	rrtemporal "github.com/temporalio/roadrunner-temporal/v5"

	"github.com/stretchr/testify/assert"
)

func TestTemporalCheckStatus(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	cfg := &config.Plugin{
		Version: "2023.3.0",
		Path:    "../configs/.rr-status.yaml",
	}

	err := cont.RegisterAll(
		cfg,
		&status.Plugin{},
		&logger.Plugin{},
		&rrtemporal.Plugin{},
		&server.Plugin{},
	)
	assert.NoError(t, err)

	err = cont.Init()
	if err != nil {
		t.Fatal(err)
	}

	ch, err := cont.Serve()
	assert.NoError(t, err)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	}()

	time.Sleep(time.Second)

	client := &http.Client{
		Timeout: time.Second * 10,
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:35544/health?plugin=temporal", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "plugin: temporal, status: 200\n", string(body))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	req, err = http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:35544/ready?plugin=temporal", nil)
	require.NoError(t, err)

	resp, err = client.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	body, _ = io.ReadAll(resp.Body)
	assert.Equal(t, "plugin: temporal, status: 200\n", string(body))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	stopCh <- struct{}{}

	wg.Wait()
}
