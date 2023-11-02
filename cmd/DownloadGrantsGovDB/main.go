// Package main compiles to an AWS Lambda handler binary that, when invoked, downloads
// the Grants.gov database export for the date specified in the "timestamp" field of the
// invocation event payload. The Lambda function streams the database export file to an
// object the S3 bucket named by the GRANTS_SOURCE_DATA_BUCKET_NAME environment variable.
// The resulting S3 object is keyed as "sources/YYYY/mm/dd/grants.gov/archive.zip", where
// the "YYYY/mm/dd" path components represent the date of the database export.
package main

import (
	"context"
	"fmt"
	goLog "log"
	"net/http"
	"time"

	ddlambda "github.com/DataDog/datadog-lambda-go"
	goenv "github.com/Netflix/go-env"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/usdigitalresponse/grants-ingest/internal/awsHelpers"
	"github.com/usdigitalresponse/grants-ingest/internal/ddHelpers"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	awstrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/aws/aws-sdk-go-v2/aws"
	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
)

type Environment struct {
	LogLevel           string        `env:"LOG_LEVEL,default=INFO"`
	DestinationBucket  string        `env:"GRANTS_SOURCE_DATA_BUCKET_NAME,required=true"`
	GrantsGovBaseURL   string        `env:"GRANTS_GOV_BASE_URL,required=true"`
	GrantsGovPathURL   string        `env:"GRANTS_GOV_PATH_URL,required=true"`
	MaxDownloadBackoff time.Duration `env:"MAX_DOWNLOAD_BACKOFF,default=20s"`
	UsePathStyleS3Opt  bool          `env:"S3_USE_PATH_STYLE,default=false"`
	Extras             goenv.EnvSet
}

var (
	env        Environment
	logger     log.Logger
	sendMetric = ddHelpers.NewMetricSender("DownloadGrantsGovDB", "source:grants.gov")
)

func main() {
	es, err := goenv.UnmarshalFromEnviron(&env)
	if err != nil {
		goLog.Fatalf("error configuring environment variables: %v", err)
	}
	env.Extras = es
	log.ConfigureLogger(&logger, env.LogLevel)

	log.Debug(logger, "Starting Lambda")
	lambda.Start(ddlambda.WrapFunction(func(ctx context.Context, event ScheduledEvent) error {
		cfg, err := awsHelpers.GetConfig(ctx)
		if err != nil {
			return fmt.Errorf("could not create AWS SDK config: %w", err)
		}
		awstrace.AppendMiddleware(&cfg)
		httptrace.WrapClient(http.DefaultClient)
		return handleWithConfig(cfg, ctx, event)
	}, nil))
}
