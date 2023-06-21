// Package main compiles to an AWS Lambda handler binary that, when invoked, downloads
// and extracts the zip archive file object specified in the invocation event from S3.
// The XML file extracted from the zip archive is then uploaded back to S3, first to
// an intermediary destination (of "$TMP_KEY_PATH_PREFIX/<archive base path>/extract.xml").
// It then verifies that no unexpected content is provided by the zip file (suggesting that
// the contents from Grants.gov may contain breaking changes). Upon successful verification,
// the extracted XML file object is moved to its permanent S3 destination, with the same base
// path (key prefix) of the incoming zip archive, but with a file name (key suffix) of "extract.xml",
// i.e. "<archive base path>/extract.xml".
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
	LogLevel          string `env:"LOG_LEVEL,default=INFO"`
	UsePathStyleS3Opt bool   `env:"S3_USE_PATH_STYLE,default=false"`
	TmpKeyPrefix      string `env:"TMP_KEY_PATH_PREFIX,default=tmp"`
	Extras            goenv.EnvSet
	// Should use zero (default) except during testing or performance tuning
	DownloadPartSize int64 `env:"DOWNLOAD_PART_SIZE,default=0"`
}

var (
	env        Environment
	logger     log.Logger
	sendMetric = ddHelpers.NewMetricSender("ExtractGrantsGovDBToXML", "source:grants.gov")
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
		s3svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.UsePathStyle = env.UsePathStyleS3Opt
		})
		log.Debug(logger, "Starting Lambda inner")
		return handleS3Event(ctx, s3svc, s3Event)
	}, nil))
}
