package s3coordinator

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/transferia/transferia/internal/logger"
	"github.com/transferia/transferia/library/go/core/xerrors"
	"github.com/transferia/transferia/tests/tcrecipes"
	"github.com/transferia/transferia/tests/tcrecipes/objectstorage"
	"go.ytsaurus.tech/library/go/core/log"
)

func envOrDefault(key, def string) string {
	if r, ok := os.LookupEnv(key); ok {
		return r
	}
	return def
}

func NewS3Recipe(bucket string) (*CoordinatorS3, error) {
	ctx := context.Background()
	if tcrecipes.Enabled() || os.Getenv("S3_ENDPOINT") == "" {
		_, err := objectstorage.Prepare(ctx)
		if err != nil {
			return nil, xerrors.Errorf("unable to prepare recipe: %w", err)
		}
	}
	// infer args from env
	defaultEndpoint := "http://localhost:9000"
	if port := os.Getenv("S3MDS_PORT"); port != "" {
		defaultEndpoint = fmt.Sprintf("http://localhost:%s", port)
	}
	endpoint := envOrDefault("S3_ENDPOINT", defaultEndpoint)
	region := envOrDefault("S3_REGION", "ru-central1")
	accessKey := envOrDefault("S3_ACCESS_KEY", "1234567890")
	secret := envOrDefault("S3_SECRET", "abcdefabcdef")

	baseCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secret, "")),
	)
	if err != nil {
		return nil, xerrors.Errorf("unable to init aws config: %w", err)
	}
	client := s3.NewFromConfig(baseCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	if bucket == "" {
		bucket = "coordinator"
		res, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(bucket),
		})
		// No need to check error because maybe the bucket already exists
		logger.Log.Info("create bucket result", log.Any("res", res), log.Error(err))
	}
	cp, err := NewS3(
		bucket,
		logger.Log,
		baseCfg,
		func(o *s3.Options) {
			o.UsePathStyle = true
			o.BaseEndpoint = aws.String(endpoint)
		},
	)
	if err != nil {
		return nil, xerrors.Errorf("unable to create s3 coordinator: %w", err)
	}
	return cp, nil
}
