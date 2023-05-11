package main

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
)

type S3API interface {
	GetObject(ctx context.Context,
		params *s3.GetObjectInput,
		optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

func handleS3Event(ctx context.Context, s3Event events.S3Event, s3client S3API) error {
	uploadedFile := s3Event.Records[0].S3.Object.Key
	log.Info(logger, "Received S3 event", "uploadedFile", uploadedFile)
	return nil
}
