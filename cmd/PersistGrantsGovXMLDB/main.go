// Package main compiles to an AWS Lambda handler binary that, when invoked, reads an XML file
// that contains an OpportunityDetail-V1.0 object from a source S3 object identified by the
// S3:ObjectCreated:* invocation event payload. While reading the XML data, each tag/value is
// uploaded to a DynamoDB table identified by the GRANTS_PREPARED_DYNAMODB_NAME
// environment variable with the primary hash key (grant_id) being the OpportunityID value.
package main

import (
	"context"
	"fmt"
	goLog "log"

	ddlambda "github.com/DataDog/datadog-lambda-go"
	goenv "github.com/Netflix/go-env"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/usdigitalresponse/grants-ingest/internal/awsHelpers"
	"github.com/usdigitalresponse/grants-ingest/internal/ddHelpers"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	awstrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/aws/aws-sdk-go-v2/aws"
)

type Environment struct {
	LogLevel          string `env:"LOG_LEVEL,default=INFO"`
	DestinationTable  string `env:"GRANTS_PREPARED_DYNAMODB_NAME,required=true"`
	UsePathStyleS3Opt bool   `env:"S3_USE_PATH_STYLE,default=false"`
	Extras            goenv.EnvSet
}

var (
	env        Environment
	logger     log.Logger
	sendMetric = ddHelpers.NewMetricSender("PersistGrantsGovXMLDB", "source:grants.gov")
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

		// Configure service clients
		s3Svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.UsePathStyle = env.UsePathStyleS3Opt
		})
		dynamodbSvc := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {})

		log.Debug(logger, "Starting Lambda")
		return handleS3EventWithConfig(s3Svc, dynamodbSvc, ctx, s3Event)
	}, nil))
}
