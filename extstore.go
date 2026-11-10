package rrtemporal

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/roadrunner-server/errors"
	"go.temporal.io/sdk/contrib/aws/s3driver"
	"go.temporal.io/sdk/contrib/aws/s3driver/awssdkv2"
	"go.temporal.io/sdk/converter"
)

const externalStorageDriverS3 string = "s3"

type ExternalStorage struct {
	Driver               string `mapstructure:"driver"`
	Name                 string `mapstructure:"name"`
	PayloadSizeThreshold int    `mapstructure:"payload_size_threshold"`
	Bucket               string `mapstructure:"bucket"`
	Region               string `mapstructure:"region"`
	Endpoint             string `mapstructure:"endpoint"`
	ForcePathStyle       bool   `mapstructure:"force_path_style"`
}

func (e *ExternalStorage) validate() error {
	const op = errors.Op("external_storage_validate")

	if e.Driver == "" {
		e.Driver = externalStorageDriverS3
	}

	if e.Driver != externalStorageDriverS3 {
		return errors.E(op, errors.Errorf("unsupported external storage driver %q, supported: %q", e.Driver, externalStorageDriverS3))
	}

	if e.Bucket == "" {
		return errors.E(op, errors.Str("external storage bucket must not be empty"))
	}

	if e.PayloadSizeThreshold < 0 {
		return errors.E(op, errors.Str("external storage payload_size_threshold must not be negative"))
	}

	return nil
}

func newExternalStorage(ctx context.Context, cfg *ExternalStorage) (converter.ExternalStorage, error) {
	const op = errors.Op("external_storage_init")

	var loadOpts []func(*awsconfig.LoadOptions) error
	if cfg.Region != "" {
		loadOpts = append(loadOpts, awsconfig.WithRegion(cfg.Region))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return converter.ExternalStorage{}, errors.E(op, err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
	})

	driver, err := s3driver.NewDriver(s3driver.Options{
		Client:     awssdkv2.NewClient(client),
		Bucket:     s3driver.StaticBucket(cfg.Bucket),
		DriverName: cfg.Name,
	})
	if err != nil {
		return converter.ExternalStorage{}, errors.E(op, err)
	}

	return converter.ExternalStorage{
		Drivers:              []converter.StorageDriver{driver},
		PayloadSizeThreshold: cfg.PayloadSizeThreshold,
	}, nil
}
