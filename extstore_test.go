package rrtemporal

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/temporalio/roadrunner-temporal/v6/api"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

func TestConfigInitDefault_ExternalStorage(t *testing.T) {
	cases := map[string]struct {
		storage *ExternalStorage
		err     string
	}{
		"negative threshold": {
			storage: &ExternalStorage{PayloadSizeThreshold: -1, S3: &S3Storage{Bucket: "payloads"}},
			err:     "external_storage.payload_size_threshold must not be negative",
		},
		"s3 without section": {
			storage: &ExternalStorage{},
			err:     "external_storage.s3.bucket must be set for the s3 driver",
		},
		"s3 without bucket": {
			storage: &ExternalStorage{S3: &S3Storage{Region: "us-east-1"}},
			err:     "external_storage.s3.bucket must be set for the s3 driver",
		},
		"s3 section with another driver": {
			storage: &ExternalStorage{Driver: "custom", S3: &S3Storage{Bucket: "payloads"}},
			err:     `external_storage.s3 is set, but the driver is "custom"`,
		},
		"endpoint without scheme": {
			storage: &ExternalStorage{S3: &S3Storage{Bucket: "payloads", Endpoint: "minio:9000"}},
			err:     `external_storage.s3.endpoint must be an absolute http or https URL, got "minio:9000"`,
		},
		"key without secret": {
			storage: &ExternalStorage{S3: &S3Storage{Bucket: "payloads", Key: "key"}},
			err:     "external_storage.s3.key and external_storage.s3.secret must be set together",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := (&Config{ExternalStorage: tc.storage}).InitDefault()

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.err)
		})
	}
}

func TestConfigInitDefault_ExternalStorageDefaultsToS3(t *testing.T) {
	cfg := &Config{ExternalStorage: &ExternalStorage{S3: &S3Storage{Bucket: "payloads", Endpoint: "http://127.0.0.1:9000"}}}

	require.NoError(t, cfg.InitDefault())
	assert.Equal(t, "s3", cfg.ExternalStorage.Driver)
}

func TestConfigInitDefault_ExternalStorageWithRegisteredDriver(t *testing.T) {
	cfg := &Config{ExternalStorage: &ExternalStorage{Driver: "custom"}}

	require.NoError(t, cfg.InitDefault())
}

func TestNewS3StorageDriver_RequiresRegion(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/missing")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", t.TempDir()+"/missing")

	_, err := newS3StorageDriver(&S3Storage{Bucket: "payloads"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "external_storage.s3.region is not set")
}

func TestNewS3StorageDriver_StoresAndRetrievesThroughTheEndpoint(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	backend := s3mem.New()
	require.NoError(t, backend.CreateBucket("payloads"))
	server := httptest.NewServer(gofakes3.New(backend).Server())
	defer server.Close()

	driver, err := newS3StorageDriver(&S3Storage{
		DriverName:     "custom.s3",
		Bucket:         "payloads",
		Region:         "us-east-1",
		Endpoint:       server.URL,
		ForcePathStyle: true,
		Key:            "key",
		Secret:         "secret",
	})
	require.NoError(t, err)
	assert.Equal(t, "custom.s3", driver.Name())

	payload := &commonpb.Payload{Metadata: map[string][]byte{"encoding": []byte("json/plain")}, Data: []byte(`"value"`)}
	claims, err := driver.Store(converter.StorageDriverStoreContext{Context: context.Background()}, []*commonpb.Payload{payload})
	require.NoError(t, err)
	require.Len(t, claims, 1)

	objects, err := backend.ListBucket("payloads", nil, gofakes3.ListBucketPage{})
	require.NoError(t, err)
	require.Len(t, objects.Contents, 1)
	assert.Equal(t, claims[0].ClaimData["key"], objects.Contents[0].Key)

	payloads, err := driver.Retrieve(converter.StorageDriverRetrieveContext{Context: context.Background()}, claims)
	require.NoError(t, err)
	require.Len(t, payloads, 1)
	assert.Equal(t, payload.GetData(), payloads[0].GetData())
}

type testStorageDriver struct {
	driver converter.StorageDriver
}

func (testStorageDriver) Name() string { return "custom" }

func (d testStorageDriver) StorageDriver() converter.StorageDriver { return d.driver }

func TestInitExternalStorage_UsesRegisteredDriver(t *testing.T) {
	driver, err := newS3StorageDriver(&S3Storage{Bucket: "payloads", Region: "us-east-1", DriverName: "registered"})
	require.NoError(t, err)
	p := testPluginWithStorage(&ExternalStorage{Driver: "custom", PayloadSizeThreshold: 1024})
	p.temporal.storageDrivers["custom"] = testStorageDriver{driver: driver}

	require.NoError(t, p.initExternalStorage())
	require.Equal(t, []converter.StorageDriver{driver}, p.temporal.externalStorage.Drivers)
	assert.Equal(t, 1024, p.temporal.externalStorage.PayloadSizeThreshold)
}

func TestInitExternalStorage_FailsOnUnknownDriver(t *testing.T) {
	p := testPluginWithStorage(&ExternalStorage{Driver: "missing"})

	err := p.initExternalStorage()

	require.Error(t, err)
	assert.Contains(t, err.Error(), `external storage driver "missing" is not registered`)
}

func TestInitExternalStorage_DisabledWithoutConfig(t *testing.T) {
	p := testPluginWithStorage(nil)

	require.NoError(t, p.initExternalStorage())
	assert.Empty(t, p.temporal.externalStorage.Drivers)
}

func testPluginWithStorage(storage *ExternalStorage) *Plugin {
	return &Plugin{
		config:   &Config{ExternalStorage: storage},
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		temporal: &temporal{storageDrivers: make(map[string]api.StorageDriver)},
	}
}
