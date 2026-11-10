package rrtemporal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExternalStorageValidate_DefaultsToS3(t *testing.T) {
	cfg := &ExternalStorage{Bucket: "payloads"}

	require.NoError(t, cfg.validate())
	assert.Equal(t, "s3", cfg.Driver)
}

func TestExternalStorageValidate_RejectsInvalidConfig(t *testing.T) {
	cases := map[string]*ExternalStorage{
		"unknown driver":     {Driver: "gcs", Bucket: "payloads"},
		"empty bucket":       {Driver: "s3"},
		"negative threshold": {Bucket: "payloads", PayloadSizeThreshold: -1},
	}

	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, cfg.validate())
		})
	}
}

func TestConfigInitDefault_ValidatesExternalStorage(t *testing.T) {
	cfg := &Config{ExternalStorage: &ExternalStorage{}}

	assert.Error(t, cfg.InitDefault())
}

func TestNewExternalStorage_BuildsS3Driver(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	storage, err := newExternalStorage(context.Background(), &ExternalStorage{
		Name:                 "custom.s3",
		Bucket:               "payloads",
		Region:               "us-east-1",
		Endpoint:             "http://127.0.0.1:9000",
		ForcePathStyle:       true,
		PayloadSizeThreshold: 1024,
	})

	require.NoError(t, err)
	require.Len(t, storage.Drivers, 1)
	assert.Equal(t, "custom.s3", storage.Drivers[0].Name())
	assert.Equal(t, "aws.s3driver", storage.Drivers[0].Type())
	assert.Equal(t, 1024, storage.PayloadSizeThreshold)
}
