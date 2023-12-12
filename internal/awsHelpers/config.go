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
	useLocalStack, lsHostname, lsPort := getLocalStackEndpoint()
	if !useLocalStack {
		return config.LoadDefaultConfig(ctx)
	}

	optionsFunc := func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		awsEndpoint := aws.Endpoint{
			PartitionID: "aws",
			URL:         fmt.Sprintf("http://%s:%s", lsHostname, lsPort),
		}
		if awsRegion, isSet := os.LookupEnv("AWS_REGION"); isSet {
			awsEndpoint.SigningRegion = awsRegion
		}
		return awsEndpoint, nil
	}
	customResolver := aws.EndpointResolverWithOptionsFunc(optionsFunc)
	return config.LoadDefaultConfig(ctx, config.WithEndpointResolverWithOptions(customResolver))
}

func GetSQSClient(ctx context.Context) (*sqs.Client, error) {
	cfg, err := GetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not create AWS SDK config: %w", err)
	}

	useLocalStack, lsHostname, lsPort := getLocalStackEndpoint()
	if !useLocalStack {
		return sqs.NewFromConfig(cfg), nil
	}

	return sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("http://%s:%s", lsHostname, lsPort))
	}), nil
}

func getLocalStackEndpoint() (isConfigured bool, hostname, port string) {
	hostname, isConfigured = os.LookupEnv("LOCALSTACK_HOSTNAME")
	if isConfigured {
		port = "4566"
		if lsPort, isSet := os.LookupEnv("EDGE_PORT"); isSet {
			port = lsPort
		}
	}

	return isConfigured, hostname, port
}
