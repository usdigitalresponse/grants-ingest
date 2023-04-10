package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
)

const (
	MB                         = int64(1024 * 1024)
	GRANT_OPPORTUNITY_XML_NAME = "OpportunitySynopsisDetail_1_0"
)

func handleS3EventWithConfig(cfg aws.Config, ctx context.Context, s3Event events.S3Event) error {
	fmt.Print("Hello, ")
	fmt.Print("World!")

	return nil
}
