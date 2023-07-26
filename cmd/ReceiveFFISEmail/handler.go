package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
)

type S3API interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
}

func handleEvent(ctx context.Context, client S3API, event events.S3Event) error {
	sourceBucket := event.Records[0].S3.Bucket.Name
	sourceKey := event.Records[0].S3.Object.Key
	logger := log.With(logger, "source_bucket", sourceBucket, "source_key", sourceKey,
		"destination_bucket", env.DestinationBucket)

	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(sourceBucket),
		Key:    aws.String(sourceKey),
	})
	if err != nil {
		return log.Errorf(logger, "failed to retrieve S3 object", err)
	}
	defer resp.Body.Close()

	msg, sender, sentAt, err := parseEmailContents(resp.Body)
	if err != nil {
		return log.Errorf(logger, "failed to parse email from S3 object", err)
	}
	logger = log.With(logger,
		"email_sender_name", sender.Name, "email_sender_address", sender.Address)

	if err := verifyEmailIsTrusted(msg, sender); err != nil {
		sendMetric("email.untrusted", 1)
		return log.Errorf(logger, "email cannot be trusted", err)
	}

	destKey := fmt.Sprintf("sources/%s/ffis.org/raw.eml", sentAt.Format("2006/01/02"))
	logger = log.With(logger, "destination_key", destKey, "email_date", sentAt)
	if _, err := client.CopyObject(ctx, &s3.CopyObjectInput{
		CopySource:           aws.String(filepath.Join(sourceBucket, sourceKey)),
		Bucket:               aws.String(env.DestinationBucket),
		Key:                  aws.String(destKey),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	}); err != nil {
		return log.Errorf(logger, "failed to copy S3 object", err)
	}

	log.Info(logger, "Successfully copied email to destination bucket")
	return nil
}
