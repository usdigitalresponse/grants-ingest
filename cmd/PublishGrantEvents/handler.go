package main

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
)

func handleWithConfig(cfg aws.Config, ctx context.Context, event events.DynamoDBEvent) error {
	log.Info(logger, "In the handler")

	return nil
}
