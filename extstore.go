package rrtemporal

import (
	"context"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/roadrunner-server/errors"
	"go.temporal.io/sdk/contrib/aws/s3driver"
	"go.temporal.io/sdk/contrib/aws/s3driver/awssdkv2"
	"go.temporal.io/sdk/converter"
)

const externalStorageDriverS3 string = "s3"

type ExternalStorage struct {
	Driver               string     `mapstructure:"driver"`
	PayloadSizeThreshold int        `mapstructure:"payload_size_threshold"`
	S3                   *S3Storage `mapstructure:"s3"`
}

type S3Storage struct {
	DriverName     string `mapstructure:"driver_name"`
	Bucket         string `mapstructure:"bucket"`
	Region         string `mapstructure:"region"`
	Endpoint       string `mapstructure:"endpoint"`
	ForcePathStyle bool   `mapstructure:"force_path_style"`
	Key            string `mapstructure:"key"`
	Secret         string `mapstructure:"secret"`
}

func newS3StorageDriver(cfg *S3Storage) (converter.StorageDriver, error) {
	const op = errors.Op("temporal_external_storage_s3_init")

	var opts []func(*awsconfig.LoadOptions) error
	if cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.Region))
	}
	if cfg.Key != "" && cfg.Secret != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.Key, cfg.Secret, "")))
	}
	if cfg.Endpoint != "" {
		opts = append(opts, awsconfig.WithBaseEndpoint(cfg.Endpoint))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, errors.E(op, err)
	}
	if awsCfg.Region == "" {
		return nil, errors.E(op, errors.Str("external_storage.s3.region is not set and the AWS configuration has no region"))
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.ForcePathStyle
	})

	driver, err := s3driver.NewDriver(s3driver.Options{
		Client:     awssdkv2.NewClient(client),
		Bucket:     s3driver.StaticBucket(cfg.Bucket),
		DriverName: cfg.DriverName,
	})
	if err != nil {
		return nil, errors.E(op, err)
	}

	return driver, nil
}

func (p *Plugin) initExternalStorage() error {
	const op = errors.Op("temporal_external_storage_init")

	cfg := p.config.ExternalStorage
	if cfg == nil {
		return nil
	}

	driver := p.temporal.s3Driver
	if cfg.Driver != externalStorageDriverS3 {
		provider, ok := p.temporal.storageDrivers[cfg.Driver]
		if !ok {
			return errors.E(op, errors.Errorf("external storage driver %q is not registered", cfg.Driver))
		}
		driver = provider.StorageDriver()
	}

	p.temporal.externalStorage = converter.ExternalStorage{
		Drivers:              []converter.StorageDriver{driver},
		PayloadSizeThreshold: cfg.PayloadSizeThreshold,
	}

	return nil
}
