package tests

import (
	"context"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"tests/helpers"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/aws/s3driver"
	"go.temporal.io/sdk/contrib/aws/s3driver/awssdkv2"
	"go.temporal.io/sdk/converter"
	"google.golang.org/protobuf/encoding/protojson"
)

const externalStorageReferenceType = "temporal.api.sdk.v1.ExternalStorageReference"

func Test_ExternalStorage_OffloadsAndResolvesPayloads(t *testing.T) {
	backend := s3mem.New()
	require.NoError(t, backend.CreateBucket("payloads"))
	fakeS3 := httptest.NewServer(gofakes3.New(backend).Server())
	defer fakeS3.Close()
	t.Setenv("S3_ENDPOINT", fakeS3.URL)

	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := helpers.NewTestServerWithExternalStorage(t, stopCh, wg, "../configs/.rr-external-storage.yaml", externalStorage(t, fakeS3.URL))

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	input := strings.Repeat("x", 4096)
	w, err := s.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{TaskQueue: "default"}, "ExternalStorageWorkflow", input)
	require.NoError(t, err)

	var result string
	require.NoError(t, w.Get(ctx, &result))
	assert.Equal(t, input, result)

	resp, err := s.Client.WorkflowService().GetWorkflowExecutionHistory(ctx, &workflowservice.GetWorkflowExecutionHistoryRequest{
		Namespace: "default",
		Execution: &commonpb.WorkflowExecution{WorkflowId: w.GetID(), RunId: w.GetRunID()},
	})
	require.NoError(t, err)

	references := map[string]bool{}
	for _, event := range resp.GetHistory().GetEvents() {
		switch {
		case event.GetWorkflowExecutionStartedEventAttributes() != nil:
			references["workflow input"] = isReference(event.GetWorkflowExecutionStartedEventAttributes().GetInput())
		case event.GetActivityTaskScheduledEventAttributes() != nil:
			references["activity input"] = isReference(event.GetActivityTaskScheduledEventAttributes().GetInput())
		case event.GetActivityTaskCompletedEventAttributes() != nil:
			references["activity result"] = isReference(event.GetActivityTaskCompletedEventAttributes().GetResult())
		case event.GetWorkflowExecutionCompletedEventAttributes() != nil:
			references["workflow result"] = isReference(event.GetWorkflowExecutionCompletedEventAttributes().GetResult())
		}
	}
	assert.Equal(t, map[string]bool{
		"workflow input":  true,
		"activity input":  true,
		"activity result": true,
		"workflow result": true,
	}, references)

	objects, err := backend.ListBucket("payloads", nil, gofakes3.ListBucketPage{})
	require.NoError(t, err)
	assert.NotEmpty(t, objects.Contents)

	rawHistory, err := protojson.Marshal(resp.GetHistory())
	require.NoError(t, err)
	historyPath := path.Join(t.TempDir(), "raw-history.json")
	require.NoError(t, os.WriteFile(historyPath, rawHistory, 0o600))
	replay := helpers.ReplayFromJSONResponse(t, "127.0.0.1:6001", historyPath, "ExternalStorageWorkflow")
	assert.Zero(t, replay.GetStatus().GetCode(), replay.GetStatus().GetMessage())

	stopCh <- struct{}{}
	wg.Wait()
	time.Sleep(time.Second)
}

func isReference(payloads *commonpb.Payloads) bool {
	return string(payloads.GetPayloads()[0].GetMetadata()["messageType"]) == externalStorageReferenceType
}

func externalStorage(t *testing.T, endpoint string) converter.ExternalStorage {
	s3Client := s3.New(s3.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider("test", "test", ""),
	})

	driver, err := s3driver.NewDriver(s3driver.Options{
		Client: awssdkv2.NewClient(s3Client),
		Bucket: s3driver.StaticBucket("payloads"),
	})
	require.NoError(t, err)

	return converter.ExternalStorage{
		Drivers:              []converter.StorageDriver{driver},
		PayloadSizeThreshold: 1024,
	}
}
