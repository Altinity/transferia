package rolechain

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func newConfig(
	ctx context.Context,
	region string,
	creds aws.CredentialsProvider,
) (aws.Config, error) {
	return awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(creds),
	)
}

func singleStep(
	ctx context.Context,
	cfg aws.Config,
	roleArn string,
) (aws.Config, error) {
	assumeRoleProvider := stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), roleArn)
	return newConfig(
		ctx,
		cfg.Region,
		aws.NewCredentialsCache(assumeRoleProvider),
	)
}

// NewConfig allows creating aws.Config using multiple role assumptions.
// For example: RoleA assumes RoleB, RoleB assumes RoleC.
func NewConfig(
	ctx context.Context,
	cfg aws.Config,
	roles ...string,
) (aws.Config, error) {
	for _, role := range roles {
		nextCfg, err := singleStep(ctx, cfg, role)
		if err != nil {
			return aws.Config{}, err
		}
		cfg = nextCfg
	}
	return cfg, nil
}

// NewSession is kept for compatibility with previous naming.
func NewSession(
	ctx context.Context,
	cfg aws.Config,
	roles ...string,
) (aws.Config, error) {
	return NewConfig(ctx, cfg, roles...)
}
