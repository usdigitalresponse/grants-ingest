// Package main compiles to an AWS Lambda handler binary that, when invoked, ingests
// DynamoDB stream events, publishing each record as a new event to the "Grants"
// EventBridge event bus. 
// On error, sends failing events to the "Publish Grant Events DLQ" dead-letter queue.
// Keeps track of the SequenceNumber attributes of events that fail to publish to EventBridge,
// and reports them at the end of each invocation.
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
	"github.com/aws/aws-lambda-go/events"
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
	lambda.Start(ddlambda.WrapFunction(func(ctx context.Context, event events.DynamoDBEvent) error {
		cfg, err := awsHelpers.GetConfig(ctx)
		if err != nil {
			return fmt.Errorf("could not create AWS SDK config: %w", err)
		}
		awstrace.AppendMiddleware(&cfg)
		httptrace.WrapClient(http.DefaultClient)
		return handleWithConfig(cfg, ctx, event)
	}, nil))
}
