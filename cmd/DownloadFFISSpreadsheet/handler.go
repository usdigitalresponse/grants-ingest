package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	ffis "github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/ffis"
)

type SQSAPI interface {
	ReceiveMessage(ctx context.Context,
		params *sqs.ReceiveMessageInput,
		optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
}

type S3API interface {
	PutObject(ctx context.Context,
		params *s3.PutObjectInput,
		optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)

	CreateMultipartUpload(ctx context.Context,
		params *s3.CreateMultipartUploadInput,
		optFns ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	AbortMultipartUpload(ctx context.Context,
		params *s3.AbortMultipartUploadInput,
		optFns ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)

	CompleteMultipartUpload(ctx context.Context,
		params *s3.CompleteMultipartUploadInput,
		optFns ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)

	UploadPart(ctx context.Context,
		params *s3.UploadPartInput,
		optFns ...func(*s3.Options)) (*s3.UploadPartOutput, error)
}

func handleSQSEvent(ctx context.Context, sqsEvent events.SQSEvent, s3client S3API, sqsclient SQSAPI) error {
	msg := sqsEvent.Records[0].Body
	log.Info(logger, "Received message", "message", msg)
	var ffisMessage ffis.FFISMessageDownload
	err := json.Unmarshal([]byte(msg), &ffisMessage)
	if err != nil {
		return fmt.Errorf("error unmarshalling SQS message: %w", err)
	}

	fileStr, err := parseSQSMessage(ffisMessage)
	if err != nil {
		return fmt.Errorf("error parsing SQS message: %w", err)
	}
	defer fileStr.Close()
	err = writeToS3(ctx, s3client, fileStr, ffisMessage.SourceFileKey)
	return err
}

func parseSQSMessage(msg ffis.FFISMessageDownload) (stream io.ReadCloser, err error) {
	httpClient := http.Client{}
	resp, err := httpClient.Get(msg.DownloadURL)
	if err != nil {
		return nil, fmt.Errorf("error downloading file: %w", err)
	}
	log.Debug(logger, "Downloaded file", "url", msg.DownloadURL)
	return resp.Body, nil
}

func writeToS3(ctx context.Context, s3client S3API, fileStr io.ReadCloser, sourceKey string) error {
	log.Info(logger, "Writing to S3", "sourceKey", sourceKey, "destinationBucket", env.DestinationBucket)
	_, err := s3manager.NewUploader(s3client).Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(env.DestinationBucket),
		Key:    aws.String("ffis/test.xlsx"), // TODO
		Body:   fileStr,
	})
	return err
}
