package awsHelpers

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// GetConfig returns an AWS SDK v2 Config with a custom resolver that resolves SDK requests
// to an endpoint at http://$LOCALSTACK_HOSTNAME:4566 when $LOCALSTACK_HOSTNAME is configured
// in the current environment.
// $EDGE_PORT will override port 4566 only when $LOCALSTACK_HOSTNAME is also set.
// If no $LOCALSTACK_HOSTNAME variable exists in the current environment, the resolver falls
// back to the SDK's default endpoint resolution behavior.
func GetConfig(ctx context.Context) (aws.Config, error) {
	optionsFunc := func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if lsHostname, isSet := os.LookupEnv("LOCALSTACK_HOSTNAME"); isSet {
			lsPort := "4566"
			if edgePort, isSet := os.LookupEnv("EDGE_PORT"); isSet {
				lsPort = edgePort
			}
			awsEndpoint := fmt.Sprintf("http://%s:%s", lsHostname, lsPort)
			return aws.Endpoint{URL: awsEndpoint}, nil
		}

		// Allow fallback to default resolution
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	}
	resolver := aws.EndpointResolverWithOptionsFunc(optionsFunc)
	return config.LoadDefaultConfig(ctx, config.WithEndpointResolverWithOptions(resolver))
}

func GetSQSClient(ctx context.Context) (*sqs.Client, error) {
	cfg, err := GetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not create AWS SDK config: %w", err)
	}

	var sqsResolver sqs.EndpointResolverFunc = func(region string, options sqs.EndpointResolverOptions) (aws.Endpoint, error) {
		return cfg.EndpointResolverWithOptions.ResolveEndpoint("sqs", cfg.Region)
	}
	client := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		// the logic for providing the config above doesn't affect the endpoint for SQS, and this is
		// needed so that localstack will work
		if _, isSet := os.LookupEnv("LOCALSTACK_HOSTNAME"); isSet {
			o.EndpointResolver = sqsResolver
		}
	})

	return client, nil
}
