package kinesis

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/transferia/transferia/library/go/core/xerrors"
	tc_localstack "github.com/transferia/transferia/tests/tcrecipes/localstack"
)

func Prepare(img string) (string, error) {
	ctx := context.Background()
	net, err := network.New(ctx)
	if err != nil {
		return "", xerrors.Errorf("Failed to create network: %w", err)
	}

	res, err := tc_localstack.Run(
		ctx,
		img,
		network.WithNetwork([]string{"localstack"}, net),
		testcontainers.WithEnv(map[string]string{"SERVICES": "kinesis"}),
	)
	if err != nil {
		return "", xerrors.Errorf("Failed to run localstack container: %w", err)
	}

	endpoint, err := tc_localstack.GetEndpoint(res, ctx)
	if err != nil {
		return "", xerrors.Errorf("Failed to retrieve endpoint: %w", err)
	}

	return endpoint, nil
}

func NewClient(src *KinesisSource) (*kinesis.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(src.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			src.AccessKey,
			string(src.SecretKey),
			"",
		)),
	)
	if err != nil {
		return nil, xerrors.Errorf("failed to load aws config: %w", err)
	}

	return kinesis.NewFromConfig(cfg, func(o *kinesis.Options) {
		if src.Endpoint != "" {
			o.BaseEndpoint = aws.String(src.Endpoint)
		}
	}), nil
}

func CreateStream(streamName string, client *kinesis.Client) error {
	ctx := context.Background()
	if _, err := client.CreateStream(ctx, &kinesis.CreateStreamInput{
		StreamName: &streamName,
	}); err != nil {
		return xerrors.Errorf("Failed to create stream: %w", err)
	}

	waiter := kinesis.NewStreamExistsWaiter(client)
	if err := waiter.Wait(ctx, &kinesis.DescribeStreamInput{StreamName: &streamName}, 5*time.Minute); err != nil {
		return xerrors.Errorf("Failed to create stream: %w", err)
	}
	return nil
}

func SourceRecipe() (*KinesisSource, error) {
	endpoint, err := Prepare("localstack/localstack:2.0.0")
	if err != nil {
		return nil, xerrors.Errorf("Failed to start localstack: %w", err)
	}

	src := new(KinesisSource)
	src.Region = "us-west-2"
	src.Stream = "test_stream"
	src.AccessKey = "AKID"
	src.SecretKey = "secretkey"
	src.Endpoint = endpoint

	client, err := NewClient(src)
	if err != nil {
		return nil, xerrors.Errorf("Failed to create a Kinesis stream: %w", err)
	}

	if err = CreateStream(src.Stream, client); err != nil {
		return nil, xerrors.Errorf("Failed to create stream: %w", err)
	}

	return src, nil
}

func MustSource() *KinesisSource {
	res, err := SourceRecipe()
	if err != nil {
		panic(err)
	}
	return res
}
