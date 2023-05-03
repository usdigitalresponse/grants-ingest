package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	ffis "github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/ffis"
)

// error constants
var (
	ErrDownloadFailed = fmt.Errorf("Error downloading file")
)

type S3UploaderAPI interface {
	Upload(ctx context.Context,
		params *s3.PutObjectInput,
		optFns ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

type HTTPClientAPI interface {
	Get(url string) (resp *http.Response, err error)
}

func handleSQSEvent(ctx context.Context, sqsEvent events.SQSEvent, s3Uploader S3UploaderAPI, httpClient HTTPClientAPI) error {
	msg := sqsEvent.Records[0].Body
	log.Info(logger, "Received message", "message", msg)
	var ffisMessage ffis.FFISMessageDownload
	err := json.Unmarshal([]byte(msg), &ffisMessage)
	if err != nil {
		return fmt.Errorf("error unmarshalling SQS message: %w", err)
	}
	fileStr, err := downloadFile(ffisMessage, httpClient)
	if err != nil {
		return fmt.Errorf("error parsing SQS message: %w", err)
	}
	defer fileStr.Close()
	err = writeToS3(ctx, s3Uploader, fileStr, ffisMessage.SourceFileKey)
	return err
}

func downloadFile(msg ffis.FFISMessageDownload, httpClient HTTPClientAPI) (stream io.ReadCloser, err error) {
	resp, err := httpClient.Get(msg.DownloadURL)
	if err != nil {
		return nil, fmt.Errorf("error downloading file: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error downloading file: %w", ErrDownloadFailed)
	}
	log.Debug(logger, "Downloaded file", "url", msg.DownloadURL)
	return resp.Body, nil
}

func writeToS3(ctx context.Context, s3Uploader S3UploaderAPI, fileStr io.ReadCloser, sourceKey string) error {
	destinationKey := strings.Replace(sourceKey, "ffis/raw.eml", "ffis/download.xlsx", 1)
	log.Info(logger, "Writing to S3", "sourceKey", sourceKey, "destinationBucket", env.DestinationBucket, "destinationKey", destinationKey)
	_, err := s3Uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(env.DestinationBucket),
		Key:    &destinationKey,
		Body:   fileStr,
	})
	return err
}
