// Package main compiles to an AWS Lambda handler binary that, when invoked, streams an XML file
// conforming to the OpportunityDetail-V1.0 schema documented by Grants.gov from a source S3 object
// identified by the S3:ObjectCreated:* invocation event payload. While reading the XML source data,
// each contained grant opportunity is split into an individual object that is conditionally uploaded
// to an S3 bucket identified by the GRANTS_PREPARED_DATA_BUCKET_NAME environment variable.
// The conditional upload criteria is based on the following:
//
//   - If no object representing the source data is already present in the destination S3 bucket,
//     then it is always uploaded.
//   - If a destination object already, it will be replaced if the source data was updated more
//     recently than the destination object's creation timestamp.
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
	LogLevel                  string `env:"LOG_LEVEL,default=INFO"`
	DownloadChunkLimit        int64  `env:"DOWNLOAD_CHUNK_LIMIT,default=10"`
	DestinationBucket         string `env:"GRANTS_PREPARED_DATA_BUCKET_NAME,required=true"`
	MaxConcurrentUploads      int    `env:"MAX_CONCURRENT_UPLOADS,default=1"`
	UsePathStyleS3Opt         bool   `env:"S3_USE_PATH_STYLE,default=false"`
	IsForecastedGrantsEnabled bool   `env:"IS_FORECASTED_GRANTS_ENABLED,default=false"`
	Extras                    goenv.EnvSet
}

var (
	env        Environment
	logger     log.Logger
	sendMetric = ddHelpers.NewMetricSender("SplitGrantsGovXMLDB", "source:grants.gov")
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
