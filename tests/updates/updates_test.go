package updates

import (
	"context"
	"sync"
	"testing"
	"tests/helpers"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
)

const (
	addNameM             = "addName"
	addNameWOValidationM = "addNameWithoutValidation"
	throwExcM            = "throwException"
	randomizeNameM       = "randomizeName"
	addNameViaActivityM  = "addNameViaActivity"
	// signal
	exitSig = "exit"
	// WF names
	updateGreetWF = "Update.greet"
)

func Test_Updates(t *testing.T) {
	tests := []struct {
		name         string
		update       string
		args         []any
		want         string
		wantErr      string
		wantLen      int
		wantActivity bool
	}{
		{name: "add name", update: addNameM, args: []any{"John Doe"}, want: "Hello, John Doe!"},
		{name: "add name without validation", update: addNameWOValidationM, args: []any{"John Doe 42"}, want: "Hello, John Doe 42!"},
		{name: "validator rejects digits", update: addNameM, args: []any{"42"}, wantErr: "Name must not contain digits"},
		{name: "update throws exception", update: throwExcM, args: []any{"John Doe"}, wantErr: "Test exception with John Doe"},
		{name: "randomize one name", update: randomizeNameM, args: []any{1}, wantLen: 1},
		{name: "randomize three names", update: randomizeNameM, args: []any{3}, wantLen: 3},
		{name: "add name via activity", update: addNameViaActivityM, args: []any{"John Doe"}, want: "Hello, john doe!", wantActivity: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				UpdateName:   tt.update,
				Args:         tt.args,
				WaitForStage: client.WorkflowUpdateStageAccepted,
			})
			require.NoError(t, err)

			var result any

			err = handle.Get(context.Background(), &result)
			switch {
			case tt.wantErr != "":
				require.ErrorContains(t, err, tt.wantErr)
			case tt.wantLen > 0:
				require.NoError(t, err)
				require.Len(t, result, tt.wantLen)
			default:
				require.NoError(t, err)
				require.Equal(t, tt.want, result)
			}

			err = s.Client.SignalWorkflow(context.Background(), w.GetID(), w.GetRunID(), exitSig, nil)
			require.NoError(t, err)

			time.Sleep(time.Second)

			s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
				return event.EventType == enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED
			})

			if tt.wantActivity {
				s.AssertContainsEvent(s.Client, t, w, func(event *history.HistoryEvent) bool {
					return event.EventType == enums.EVENT_TYPE_ACTIVITY_TASK_COMPLETED
				})
			}

			stopCh <- struct{}{}
			wg.Wait()
		})
	}
}
