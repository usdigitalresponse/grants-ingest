package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/krolaw/zipstream"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
)

func fileUploadStream(ctx context.Context, m UploadManager, r io.Reader, bucket, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	logger := log.With(logger, "bucket", bucket, "destination_key", key)
	log.Debug(logger, "locating XML entry in zip stream")
	data := zipstream.NewReader(r)
	header, err := data.Next()
	if err != nil {
		return fmt.Errorf("error advancing to first entry in zip stream: %w", err)
	}
	if !strings.HasSuffix(header.Name, ".xml") {
		return fmt.Errorf("unexpected non-XML file in zip stream: %s", header.Name)
	}

	log.Debug(logger, "located start of XML file in zip stream; ready to upload")
	if _, err := m.Upload(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(bucket),
		Key:                  aws.String(key),
		Body:                 data,
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	}); err != nil {
		return log.Errorf(logger, "error uploading extracted XML to S3", err)
	}
	log.Debug(logger, "finished uploading extracted XML file stream to s3")
	sendMetric("xml.extracted", 1)

	if err := ctx.Err(); err != nil {
		return err
	}

	log.Debug(logger, "verifying zip archive is fully consumed")
	if header, err := data.Next(); err != nil && err != io.EOF {
		return fmt.Errorf("error advancing to expected end of zip stream: %w", err)
	} else if header != nil {
		return fmt.Errorf("unexpected additional file in zip archive: %s", header.Name)
	}

	return nil
}

func fileDownloadStream(ctx context.Context, m DownloadManager, w io.Writer, bucket, key string) error {
	logger := log.With(logger, "bucket", bucket, "source_key", key)

	log.Debug(logger, "downloading zip archive stream", "bucket", bucket, "source_key", key)
	_, err := m.Download(ctx, DummyWriterAt{w}, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		sendMetric("archive.downloaded", 1)
	}
	return err
}

func manageStreamingDownloadUpload(ctx context.Context, c S3UploaderDownloaderAPIClient, bucket, sourceKey, tmpKey string) error {
	logger := log.With(logger, "bucket", bucket, "source_key", sourceKey, "destination_key", tmpKey)
	ctx, cancel := context.WithCancel(ctx)
	reader, writer := io.Pipe()

	// Start the upload handler
	var uploadErr error
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer reader.Close()
		uploadErr = fileUploadStream(ctx, manager.NewUploader(c), reader, bucket, tmpKey)
		if uploadErr != nil {
			// Cancel the shared context to abort any in-progress download
			cancel()
		}
	}()

	// Download and stream source object contents to the pipe writer
	downloadErr := fileDownloadStream(ctx, NewSequentialDownloadManager(c), writer, bucket, sourceKey)
	writer.Close()
	if downloadErr != nil {
		// Cancel the shared context to abort any in-progress unzip/upload operation
		cancel()
		return log.Errorf(logger, "error streaming source object from S3", downloadErr)
	}
	log.Info(logger, "finished streaming source object from S3")

	wg.Wait()
	if uploadErr != nil {
		return log.Errorf(logger, "error uploading zip archive contexts to S3", uploadErr)
	}
	log.Info(logger, "finished uploading XML from zip archive stream")

	return nil
}

func moveS3Object(ctx context.Context, svc S3MoverAPIClient, bucket, oldKey, newKey string) error {
	logger := log.With(logger, "bucket", bucket, "source_key", oldKey, "destination_key", newKey)

	if _, err := svc.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:               aws.String(bucket),
		CopySource:           aws.String(filepath.Join(bucket, oldKey)),
		Key:                  aws.String(newKey),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	}); err != nil {
		return log.Errorf(logger, "error copying extracted XML to permanent destination", err)
	}
	log.Debug(logger, "copied extracted XML to permanent destination")
	sendMetric("xml.uploaded", 1)

	if _, err := svc.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(oldKey),
	}); err != nil {
		return log.Errorf(logger, "error deleting extracted XML from temporary destination", err)
	}
	log.Debug(logger, "deleted extracted XML from temporary destination")

	log.Info(logger, "moved extracted XML to permanent destination")
	return nil
}

// For local testing use the following event payload.
// Note: you may need to change the bucket and file name as you see fit based on what is available
// in your local environemnt.
//
//	awslocal lambda invoke \
//	  --region=us-west-2 \
//	  --function-name grants-ingest-ExtractGrantsGovDBToXML \
//	  --payload $(printf '{"Records":[{"s3":{"bucket":{"name":"grantsingest-tsh-grantssourcedata-456635181950-us-west-2"},"object":{"key":"archive.zip"}}}]}' | base64) \
//	  /dev/stdout
func handleS3Event(ctx context.Context, s3svc S3UploaderDownloaderMoverAPIClient, s3Event events.S3Event) error {
	record := s3Event.Records[0]
	bucket := record.S3.Bucket.Name
	sourceKey := record.S3.Object.Key
	log.Debug(logger, "received S3 object record from event", "bucket", bucket, "key", sourceKey)

	sourcePath, _ := filepath.Split(sourceKey)
	destinationKey := filepath.Join(sourcePath, "extract.xml")
	tmpDestinationKey := filepath.Join(env.TmpKeyPrefix, destinationKey)
	err := manageStreamingDownloadUpload(ctx, s3svc, record.S3.Bucket.Name, sourceKey, tmpDestinationKey)
	if err != nil {
		return log.Errorf(logger, "failed to stream zip archive to XML object", err)
	}

	if err := moveS3Object(ctx, s3svc, bucket, tmpDestinationKey, destinationKey); err != nil {
		return log.Errorf(logger,
			"failed to move XML upload from temporary path to permanent destination", err)
	}

	return nil
}
