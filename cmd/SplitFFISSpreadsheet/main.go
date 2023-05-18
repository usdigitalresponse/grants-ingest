// Package main compiles to an AWS Lambda handler binary that, when invoked, downloads an
// FFIS excel file from the source S3 bucket and uploads the parsed opportunities to
// as individual JSON files to the destination S3 bucket. If a row of the spreadsheet
// is not able to be parsed, the error is logged at WARN level and the row is skipped.
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
	LogLevel             string `env:"LOG_LEVEL,default=INFO"`
	DownloadChunkLimit   int64  `env:"DOWNLOAD_CHUNK_LIMIT,default=10"`
	DestinationBucket    string `env:"GRANTS_PREPARED_DATA_BUCKET_NAME,required=true"`
	MaxConcurrentUploads int    `env:"MAX_CONCURRENT_UPLOADS,default=1"`
	UsePathStyleS3Opt    bool   `env:"S3_USE_PATH_STYLE,default=false"`
	Extras               goenv.EnvSet
}

var (
	env        Environment
	logger     log.Logger
	sendMetric = ddHelpers.NewMetricSender("SplitFFISSpreadsheet", "source:ffis.org")
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
