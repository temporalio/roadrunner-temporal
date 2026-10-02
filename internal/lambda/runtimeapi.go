package lambda

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	apiVersion      = "2018-06-01"
	requestIDHeader = "Lambda-Runtime-Aws-Request-Id"
	deadlineHeader  = "Lambda-Runtime-Deadline-Ms"
	responseTimeout = time.Second
	ResponseReserve = time.Second
)

// Invocation is a single Lambda invocation handed over by the Runtime API.
type Invocation struct {
	RequestID string
	Deadline  time.Time
}

// RuntimeAPI talks to the AWS Lambda Runtime API socket provided to the function.
type RuntimeAPI struct {
	host    string
	polling *http.Client
	posting *http.Client
}

func NewRuntimeAPI(host string) *RuntimeAPI {
	return &RuntimeAPI{
		host:    host,
		polling: &http.Client{Timeout: 0},
		posting: &http.Client{Timeout: responseTimeout},
	}
}

// NextInvocation blocks until Lambda hands over an invocation.
func (a *RuntimeAPI) NextInvocation() (*Invocation, error) {
	response, err := a.polling.Get(a.url("runtime/invocation/next"))
	if err != nil {
		return nil, fmt.Errorf("fetch the next invocation: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("fetch the next invocation: HTTP %d", response.StatusCode)
	}

	requestID := response.Header.Get(requestIDHeader)
	if requestID == "" {
		return nil, fmt.Errorf("Runtime API response is missing %s", requestIDHeader)
	}

	deadlineMs, err := strconv.ParseInt(response.Header.Get(deadlineHeader), 10, 64)
	if err != nil || deadlineMs <= 0 {
		return nil, fmt.Errorf("Runtime API response is missing %s", deadlineHeader)
	}

	return &Invocation{
		RequestID: requestID,
		Deadline:  time.UnixMilli(deadlineMs),
	}, nil
}

func (a *RuntimeAPI) Respond(requestID string) error {
	return a.post(fmt.Sprintf("runtime/invocation/%s/response", requestID), "null", "send the invocation response")
}

func (a *RuntimeAPI) ReportInvocationError(requestID string, reported error) error {
	return a.post(
		fmt.Sprintf("runtime/invocation/%s/error", requestID),
		encodeError(reported),
		"report the invocation error",
	)
}

func (a *RuntimeAPI) ReportInitError(reported error) error {
	return a.post("runtime/init/error", encodeError(reported), "report the init error")
}

func (a *RuntimeAPI) post(path string, body string, what string) error {
	response, err := a.posting.Post(a.url(path), "application/json", strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("%s: HTTP %d", what, response.StatusCode)
	}

	return nil
}

func (a *RuntimeAPI) url(path string) string {
	return fmt.Sprintf("http://%s/%s/%s", a.host, apiVersion, path)
}

func encodeError(reported error) string {
	payload, err := json.Marshal(map[string]string{
		"errorType":    "RoadRunnerError",
		"errorMessage": reported.Error(),
	})
	if err != nil {
		return `{"errorType":"RoadRunnerError","errorMessage":"unencodable error"}`
	}

	return string(payload)
}
