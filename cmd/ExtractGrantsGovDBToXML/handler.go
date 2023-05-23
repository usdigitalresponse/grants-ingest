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
	"github.com/krolaw/zipstream"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
)

type DummyWriterAt struct {
	io.Writer
}

func (w DummyWriterAt) WriteAt(p []byte, offset int64) (n int, err error) {
	return w.Write(p)
}

func FileUploadStream(ctx context.Context, svc *s3.Client, r io.Reader, bucket, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data := zipstream.NewReader(r)
	header, err := data.Next()
	if err != nil {
		return fmt.Errorf("error advancing to first entry in zip stream: %w", err)
	}
	if !strings.HasSuffix(header.Name, ".xml") {
		return fmt.Errorf("unexpected non-XML file in zip stream: %s", header.Name)
	}

	if _, err := manager.NewUploader(svc).Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("tmp/" + key + header.Name),
		Body:   data,
	}); err != nil {
		log.Error(logger, "error uploading extracted XML to S3", err)
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	if header, err := data.Next(); err != nil && err != io.EOF {
		return fmt.Errorf("unexpected error advancing zip stream: %w", err)
	} else if header != nil {
		return fmt.Errorf("unexpected additional file in zip archive: %s", header.Name)
	}

	return nil
}

func FileDownloadStream(ctx context.Context, svc *s3.Client, w *io.PipeWriter, bucket, key string) error {
	_, err := manager.NewDownloader(svc).Download(ctx, DummyWriterAt{w}, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

func ManageStreamingDownloadUpload(ctx context.Context, svc *s3.Client, bucket, sourceKey, tmpKey string) error {
	logger := log.With(logger, "bucket", bucket,
		"source_key", sourceKey, "destination_key", "tmp_destination_key", tmpKey)
	log.Info(logger, "uploading XML extracted from zip archive")

	reader, writer := io.Pipe()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start the upload handler
	var uploadErr error
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer reader.Close()
		uploadErr = FileUploadStream(ctx, svc, reader, bucket, tmpKey)
	}()

	// Stream source object contents to the pipe writer
	downloader := manager.NewDownloader(svc, func(d *manager.Downloader) {
		// TODO: Do we need to initialize with `d.Concurrency = 1`?
		// d.Concurrency = 1
	})
	_, downloadErr := downloader.Download(ctx, DummyWriterAt{writer}, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
	})
	writer.Close()
	if downloadErr != nil {
		return log.Errorf(logger, "error streaming source object from S3", downloadErr)
	}
	log.Info(logger, "finished streaming source object from S3")

	wg.Wait()
	if uploadErr != nil {
		sendMetric("upload.failed", 1)
		return log.Errorf(logger, "error uploading zip archive contexts to S3", uploadErr)
	}
	log.Info(logger, "finished uploading XML from zip archive")

	return nil
}

func MoveS3Object(ctx context.Context, svc S3MoveObjectAPI, bucket, oldKey, newKey string) error {
	if _, err := svc.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(oldKey),
		Key:        aws.String(newKey),
	}); err != nil {
		return log.Errorf(logger, "error copying extracted XML to permanent destination", err)
	}

	if _, err := svc.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(oldKey),
	}); err != nil {
		return log.Errorf(logger, "error deleting extracted XML from temporary destination", err)
	}

	return nil
}

/*
For local testing use the following event payload.
Note: you may need to change the bucket and file name as you see fit based on what is available in your local environemnt.

	awslocal lambda invoke \
	    --region=us-west-2 \
	    --function-name grants-ingest-ExtractGrantsGovDBToXML \
	    --payload $(printf '{"Records": [{"eventVersion": "2.0","eventSource": "aws:s3","awsRegion": "us-west-2","eventTime": "1970-01-01T00:00:00.123Z","eventName": "ObjectCreated:Put","userIdentity": {"principalId": "EXAMPLE"},"requestParameters": {"sourceIPAddress": "127.0.0.1"},"responseElements": {"x-amz-request-id": "C3D13FE58DE4C810","x-amz-id-2": "FMyUVURIY8/IgAtTv8xRjskZQpcIZ9KG4V5Wp6S7S/JRWeUWerMUE5JgHvANOjpD"},"s3": {"s3SchemaVersion": "1.0","configurationId": "testConfigRule","bucket": {"name": “ADD_GRANTS_DATA_BUCKET_NAME,”ownerIdentity": {"principalId": "EXAMPLE"},"arn": "arn:aws:s3:::mybucket"},"object": {"key": “archive.zip”,”size": 1024,"versionId": "version","eTag": "d41d8cd98f00b204e9800998ecf8427e","sequencer": "Happy Sequencer"}}}]}' | base64) \
	    /dev/stdout
*/
func handleS3EventWithConfig(cfg aws.Config, ctx context.Context, s3Event events.S3Event) error {
	s3svc := s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = env.UsePathStyleS3Opt })

	record := s3Event.Records[0]
	bucket := record.S3.Bucket.Name
	sourceKey := record.S3.Object.Key
	log.Debug(logger, "received S3 object record from event", "bucket", bucket, "key", sourceKey)

	sourcePath, _ := filepath.Split(sourceKey)
	destinationKey := filepath.Join(sourcePath, "extract.xml")
	tmpDestinationKey := filepath.Join(env.TmpKeyPrefix, destinationKey)
	err := ManageStreamingDownloadUpload(ctx, s3svc, record.S3.Bucket.Name, sourceKey, tmpDestinationKey)
	if err != nil {
		return log.Errorf(logger, "failed to stream zip archive to XML object", err)
	}

	if err := MoveS3Object(ctx, s3svc, bucket, tmpDestinationKey, destinationKey); err != nil {
		return log.Errorf(logger,
			"failed to move XML upload from temporary path to permanent destination", err)
	}

	return nil
}
