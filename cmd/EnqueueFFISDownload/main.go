// Package main compiles to an AWS Lambda handler binary that, when invoked,
// parses the email found in the event payload for a link to the FFIS data, and
// enqueue a message to the SQS queue named by the FFIS_SQS_QUEUE_NAME environment

package main

import (
	"context"
	"fmt"
	goLog "log"

	ddlambda "github.com/DataDog/datadog-lambda-go"
	goenv "github.com/Netflix/go-env"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/usdigitalresponse/grants-ingest/internal/awsHelpers"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	awstrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/aws/aws-sdk-go-v2/aws"
)

type Environment struct {
	LogLevel            string `env:"LOG_LEVEL,default=INFO"`
	DestinationQueueURL string `env:"FFIS_SQS_QUEUE_URL,required=true"`
	UsePathStyleS3Opt   bool   `env:"S3_USE_PATH_STYLE,default=false"`
	URLPattern          string `env:"FFIS_URL_PATTERN,default=https://mcusercontent.com/.+\\.xlsx"`
	Extras              goenv.EnvSet
}

var (
	env    Environment
	logger log.Logger
	// sendMetric       = ddHelpers.NewMetricSender("EnqueueFFISDownload", "source:grants.gov")
)

func main() {
	es, err := goenv.UnmarshalFromEnviron(&env)
	if err != nil {
		goLog.Fatalf("error configuring environment variables: %v", err)
	}
	env.Extras = es
	log.ConfigureLogger(&logger, env.LogLevel)

	log.Info(logger, "Starting EnqueueFFISDownload", "destinationQueue", env.DestinationQueueURL, "urlPattern", env.URLPattern)

	log.Debug(logger, "Starting Lambda")
	lambda.Start(ddlambda.WrapFunction(func(ctx context.Context, s3Event events.S3Event) error {
		cfg, err := awsHelpers.GetConfig(ctx)
		if err != nil {
			return fmt.Errorf("could not create AWS SDK config: %w", err)
		}
		awstrace.AppendMiddleware(&cfg)
		log.Debug(logger, "Starting Lambda")
		s3client, sqsclient, err := buildClients(cfg)
		if err != nil {
			return fmt.Errorf("could not create AWS clients: %w", err)
		}
		return handleS3Event(ctx, s3Event, s3client, sqsclient)
	}, nil))
}

func buildClients(cfg aws.Config) (S3API, SQSAPI, error) {
	s3svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = env.UsePathStyleS3Opt
	})

	log.Debug(logger, "Created S3 client", "client", s3svc)

	var sqsResolver sqs.EndpointResolverFunc = func(region string, options sqs.EndpointResolverOptions) (aws.Endpoint, error) {
		return cfg.EndpointResolverWithOptions.ResolveEndpoint("sqs", cfg.Region)
	}
	sqssvc := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		// the logic in internal/awsHelpers/config doesn't affect the endpoint for SQS, and this is
		// needed so that localstack will work
		o.EndpointResolver = sqsResolver
	})

	log.Debug(logger, "Created SQS client", "client", sqssvc)

	return s3svc, sqssvc, nil

}
