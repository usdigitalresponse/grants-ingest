package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/cenkalti/backoff/v4"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// ScheduledEvent represents the invocation event for this Lambda function
type ScheduledEvent struct {
	Timestamp time.Time `json:"timestamp"`
}

// grantsURL returns the download URL for the Grants.gov database export.
func (e *ScheduledEvent) grantsURL() string {
	return fmt.Sprintf("%s/extract/GrantsDBExtract%sv2.zip",
		env.GrantsGovBaseURL,
		e.Timestamp.Format("20060102"),
	)
}

// destinationS3Key returns the S3 object key where the database export should be stored.
func (e *ScheduledEvent) destinationS3Key() string {
	return fmt.Sprintf("sources/%s/grants.gov/archive.zip", e.Timestamp.Format("2006/01/02"))
}

// handleWithConfig is a Lambda function handler that is called with the ScheduledEvent invocation
// event. When invoked, it streams a Grants.gov database export (zip file) to S3.
func handleWithConfig(cfg aws.Config, ctx context.Context, event ScheduledEvent) error {
	logger := log.With(logger,
		"db_date", event.Timestamp.Format("2006-01-02"),
		"source", event.grantsURL(),
		"destination_bucket", env.DestinationBucket,
		"destination_key", event.destinationS3Key(),
	)

	log.Debug(logger, "Starting remote file download")
	resp, err := startDownload(ctx, http.DefaultClient, event.grantsURL())
	if err != nil {
		return log.Errorf(logger, "Error initiating download request for source archive", err)
	}
	defer resp.Body.Close()
	if err := validateDownloadResponse(resp); err != nil {
		return log.Errorf(logger, "Error downloading source archive", err)
	}
	logger = log.With(logger, "source_size_bytes", resp.ContentLength)
	sendMetric("source_size", float64(resp.ContentLength))

	uploader := manager.NewUploader(s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = env.UsePathStyleS3Opt
	}))
	log.Debug(logger, "Streaming remote file to S3")
	if _, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(env.DestinationBucket),
		Key:                  aws.String(event.destinationS3Key()),
		Body:                 resp.Body,
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	}); err != nil {
		return log.Errorf(logger, "Error uploading source archive to S3", err)
	}

	log.Info(logger, "Finished transfering source file to S3")
	return nil
}

// startDownload starts a new download request and returns the response.
// Failed requests retry until env.MaxDownloadBackoff elapses.
// Returns a non-nil error if the request either could not be initialized or never succeeded.
func startDownload(ctx context.Context, c *http.Client, url string) (resp *http.Response, err error) {
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
		resp, err = c.Do(req)
		attemptSpan.Finish(tracer.WithError(err))
		return err
	}, b, func(error, time.Duration) { attempt++ })
	span.Finish(tracer.WithError(err))
	return resp, err
}

// validateDownloadResponse inspects an *http.Response and returns an error to indicate
// that the response body should not be used to create an S3 object.
func validateDownloadResponse(r *http.Response) error {
	if r.StatusCode != 200 {
		return fmt.Errorf("unexpected http response status: %s", r.Status)
	}
	if contentType := r.Header.Get("Content-Type"); contentType != "application/zip" {
		return fmt.Errorf("unexpected http response Content-Type header: %s", contentType)
	}
	return nil
}
