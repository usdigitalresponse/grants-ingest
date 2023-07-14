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

	ddlambda "github.com/DataDog/datadog-lambda-go"
	goenv "github.com/Netflix/go-env"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/usdigitalresponse/grants-ingest/internal/awsHelpers"
	"github.com/usdigitalresponse/grants-ingest/internal/ddHelpers"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	awstrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/aws/aws-sdk-go-v2/aws"
	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
)

type Environment struct {
	LogLevel     string `env:"LOG_LEVEL,default=INFO"`
	EventBusName string `env:"EVENT_BUS_NAME,required=true"`
	Extras       goenv.EnvSet
}

var (
	env        Environment
	logger     log.Logger
	sendMetric = ddHelpers.NewMetricSender("PublishGrantEvents")
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
		func(ctx context.Context, event events.DynamoDBEvent) (events.DynamoDBEventResponse, error) {
			cfg, err := awsHelpers.GetConfig(ctx)
			if err != nil {
				resp := events.DynamoDBEventResponse{}
				return resp, fmt.Errorf("could not create AWS SDK config: %w", err)
			}
			awstrace.AppendMiddleware(&cfg)
			eventBridgeClient := eventbridge.NewFromConfig(cfg)
			httptrace.WrapClient(http.DefaultClient)
			return handleEvent(ctx, eventBridgeClient, event)
		}, nil),
	)
}
