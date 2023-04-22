// Package main compiles to an AWS Lambda handler binary that, when invoked,
// parses the email found in the event payload for a link to the FFIS data, and
// enqueue a message to the SQS queue named by the FFIS_SQS_QUEUE_URL environment

package main

import (
	"context"
	"fmt"
	goLog "log"

	ddlambda "github.com/DataDog/datadog-lambda-go"
	goenv "github.com/Netflix/go-env"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/usdigitalresponse/grants-ingest/internal/awsHelpers"
	"github.com/usdigitalresponse/grants-ingest/internal/ddHelpers"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	awstrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/aws/aws-sdk-go-v2/aws"
)

type Environment struct {
	LogLevel          string `env:"LOG_LEVEL,default=INFO"`
	DestinationQueue  string `env:"FFIS_SQS_QUEUE_URL,required=true"`
	UsePathStyleS3Opt bool   `env:"S3_USE_PATH_STYLE,default=false"`
	Extras            goenv.EnvSet
}

var (
	env        Environment
	logger     log.Logger
	sendMetric = ddHelpers.NewMetricSender("EnqueueFFISDownload", "source:grants.gov")
)

func main() {
	es, err := goenv.UnmarshalFromEnviron(&env)
	if err != nil {
		goLog.Fatalf("error configuring environment variables: %v", err)
	}
	env.Extras = es
	log.ConfigureLogger(&logger, env.LogLevel)

	log.Debug(logger, "Starting Lambda")
	lambda.Start(ddlambda.WrapFunction(func(ctx context.Context, s3Event events.S3Event) error {
		cfg, err := awsHelpers.GetConfig(ctx)
		if err != nil {
			return fmt.Errorf("could not create AWS SDK config: %w", err)
		}
		awstrace.AppendMiddleware(&cfg)
		log.Debug(logger, "Starting Lambda")
		return handleS3EventWithConfig(cfg, ctx, s3Event)
	}, nil))
}
