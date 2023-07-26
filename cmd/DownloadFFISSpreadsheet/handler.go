package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/aws/aws-lambda-go/events"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	"github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/ffis"
)

// error constants
var (
	ErrDownloadFailed = fmt.Errorf("error downloading file")
)

type S3UploaderAPI interface {
	Upload(ctx context.Context,
		params *s3.PutObjectInput,
		optFns ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

type HTTPClientAPI interface {
	Do(req *http.Request) (*http.Response, error)
}

func handleSQSEvent(ctx context.Context, sqsEvent events.SQSEvent, s3Uploader S3UploaderAPI, httpClient HTTPClientAPI) error {
	msg := sqsEvent.Records[0].Body
	log.Info(logger, "Received message", "message", msg)
	var ffisMessage ffis.FFISMessageDownload
	err := json.Unmarshal([]byte(msg), &ffisMessage)
	if err != nil {
		return fmt.Errorf("error unmarshalling SQS message: %w", err)
	}
	fileStream, err := downloadFile(ctx, ffisMessage, httpClient)
	if err != nil {
		return fmt.Errorf("error parsing SQS message: %w", err)
	}
	defer fileStream.Close()
	err = writeToS3(ctx, s3Uploader, fileStream, ffisMessage.SourceFileKey)
	return err
}

func downloadFile(ctx context.Context, msg ffis.FFISMessageDownload, httpClient HTTPClientAPI) (stream io.ReadCloser, err error) {
	resp, err := startDownload(ctx, httpClient, msg.DownloadURL)
	if err != nil {
		return nil, fmt.Errorf("error downloading file: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error downloading file: %w", ErrDownloadFailed)
	}
	log.Debug(logger, "Downloaded file", "url", msg.DownloadURL)
	return resp.Body, nil
}

// startDownload starts a new download request and returns the response.
// Failed requests retry until env.MaxDownloadBackoff elapses.
// Returns a non-nil error if the request either could not be initialized or never succeeded.
func startDownload(ctx context.Context, httpClient HTTPClientAPI, url string) (resp *http.Response, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = env.MaxDownloadBackoff
	attempt := 1
	span, spanCtx := tracer.StartSpanFromContext(ctx, "download.start")
	err = backoff.RetryNotify(func() (err error) {
		attemptSpan, _ := tracer.StartSpanFromContext(spanCtx, fmt.Sprintf("attempt.%d", attempt))
		resp, err = httpClient.Do(req)
		attemptSpan.Finish(tracer.WithError(err))
		return err
	}, b, func(error, time.Duration) { attempt++ })
	span.Finish(tracer.WithError(err))
	return resp, err
}

// writeToS3 writes the contents of fileStr to the S3 bucket provied by the
// S3UploaderAPI interface.
func writeToS3(ctx context.Context, s3Uploader S3UploaderAPI, fileStream io.ReadCloser, sourceKey string) error {
	destinationKey := strings.Replace(sourceKey, "ffis.org/raw.eml", "ffis.org/download.xlsx", 1)
	log.Info(logger, "Writing to S3", "sourceKey", sourceKey, "destinationBucket", env.DestinationBucket, "destinationKey", destinationKey)
	_, err := s3Uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(env.DestinationBucket),
		Key:                  aws.String(destinationKey),
		Body:                 fileStream,
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	})
	return err
}
