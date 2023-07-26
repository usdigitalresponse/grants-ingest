// Package main compiles to an AWS Lambda handler binary that, when invoked, TBD
package main

import (
	"context"
	"fmt"
	goLog "log"

	ddlambda "github.com/DataDog/datadog-lambda-go"
	goenv "github.com/Netflix/go-env"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/usdigitalresponse/grants-ingest/internal/awsHelpers"
	"github.com/usdigitalresponse/grants-ingest/internal/ddHelpers"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	awstrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/aws/aws-sdk-go-v2/aws"
)

type Environment struct {
	LogLevel            string `env:"LOG_LEVEL,default=INFO"`
	DestinationBucket   string `env:"GRANTS_SOURCE_DATA_BUCKET_NAME,required=true"`
	UsePathStyleS3Opt   bool   `env:"S3_USE_PATH_STYLE,default=false"`
	AllowedEmailSenders string `env:"ALLOWED_EMAIL_SENDERS,required=true"`
	Extras              goenv.EnvSet
}

var (
	env        Environment
	logger     log.Logger
	sendMetric = ddHelpers.NewMetricSender("ReceiveFFISEmail")
)

func main() {
	es, err := goenv.UnmarshalFromEnviron(&env)
	if err != nil {
		goLog.Fatalf("error configuring environment variables: %v", err)
	}
	env.Extras = es
	log.ConfigureLogger(&logger, env.LogLevel)

	log.Debug(logger, "Starting Lambda")
	lambda.Start(ddlambda.WrapFunction(
		func(ctx context.Context, event events.S3Event) error {
			cfg, err := awsHelpers.GetConfig(ctx)
			if err != nil {
				return fmt.Errorf("could not create AWS SDK config: %w", err)
			}
			awstrace.AppendMiddleware(&cfg)

			s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
				o.UsePathStyle = env.UsePathStyleS3Opt
			})
			return handleEvent(ctx, s3Client, event)
		}, nil),
	)
}
