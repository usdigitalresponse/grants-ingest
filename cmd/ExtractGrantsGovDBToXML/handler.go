package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/krolaw/zipstream"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
)

type PipeWriter struct {
	io.Writer
}

type UploadResult struct {
	extractedFilekey *string
	err              error
}

func (w PipeWriter) WriteAt(p []byte, offset int64) (n int, err error) {
	fmt.Println("Writing", offset)
	return w.Write(p)
}

func FileUploadStream(ctx context.Context, uploader *manager.Uploader, pipeReader io.Reader, objectKey string, uploadResult chan<- UploadResult) {
	data := zipstream.NewReader(pipeReader)

	header, data_err := data.Next()

	if data_err != nil {
		fmt.Printf("Something went wrong %v", data_err)
		uploadResult <- UploadResult{aws.String(""), data_err}
		return
	}

	if !strings.HasSuffix(header.Name, ".xml") {
		uploadResult <- UploadResult{aws.String(""), fmt.Errorf("the following file contained within a zip is not an XML %v", header.Name)}
		return
	}

	_, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(env.GrantsDataBucket),
		Key:    aws.String("tmp/" + objectKey + header.Name),
		Body:   data,
	})

	if err != nil {
		uploadResult <- UploadResult{aws.String(""), fmt.Errorf("failed to upload extracted XML to S3 %v", err)}
		return
	}

	secondHeader, secondDataErr := data.Next()
	if secondDataErr != io.EOF {
		uploadResult <- UploadResult{aws.String(""), fmt.Errorf("expected an end of file error instead received %v", secondDataErr)}
		return
	}
	if secondHeader != nil {
		uploadResult <- UploadResult{aws.String(""), fmt.Errorf("expected only one file in the zip archivd. Received more than one. %v", secondHeader.Name)}
		return
	}

	fmt.Println("Uploader is done.")
	uploadResult <- UploadResult{aws.String("tmp/" + objectKey + header.Name), nil}
}

func FileDownloadStream(ctx context.Context, downloader *manager.Downloader, pipeWriter *io.PipeWriter, objectKey string) {
	defer pipeWriter.Close()

	_, err := downloader.Download(ctx, PipeWriter{pipeWriter}, &s3.GetObjectInput{
		Bucket: aws.String(env.GrantsDataBucket),
		Key:    aws.String(objectKey),
	})

	if err != nil {
		panic(err)
	}

	fmt.Println("Downloader is done.")
}

func ManageStreamingDownloadUpload(cfg aws.Config, objectKey string) {
	// Configure service clients
	s3svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		fmt.Println("s3 options are", o)
	})
	downloader := manager.NewDownloader(s3svc, func(d *manager.Downloader) {
		d.Concurrency = env.MaxConcurrentUploads
	})
	uploader := manager.NewUploader(s3svc)

	pipeReader, pipeWriter := io.Pipe()

	wg := sync.WaitGroup{}
	wg.Add(1)
	uploadResult := make(chan UploadResult)
	go func() {
		defer wg.Done()
		FileUploadStream(context.TODO(), uploader, pipeReader, objectKey, uploadResult)
	}()
	FileDownloadStream(context.TODO(), downloader, pipeWriter, objectKey)
	wg.Wait()

	result := <-uploadResult
	extractedFilekey := result.extractedFilekey
	extractionErr := result.err

	if extractionErr != nil {
		fmt.Println(extractionErr.Error())
		return
	}

	/* Transition file from tmp directory to main directory */
	objectPath := objectKey
	extractedFilename := extractedFilekey
	copyInput := &s3.CopyObjectInput{
		Bucket:     aws.String(env.GrantsDataBucket),
		CopySource: aws.String(*extractedFilekey),
		Key:        aws.String(objectPath + *extractedFilename),
	}

	_, copyErr := s3svc.CopyObject(context.TODO(), copyInput)
	if copyErr != nil {
		fmt.Println(copyErr.Error())
		return
	}

	/* Delete the tmp directory file */
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(env.GrantsDataBucket),
		Key:    aws.String(*extractedFilekey),
	}

	_, delErr := s3svc.DeleteObject(context.TODO(), input)
	if delErr != nil {
		fmt.Println(delErr.Error())
		return
	}
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

	for _, record := range s3Event.Records {
		fmt.Println(record)
		logger := log.With(logger)
		log.Info(logger, "Following record was identified")
		log.Info(logger, record)

		if record.S3.Bucket.Name != env.GrantsDataBucket {
			return errors.New("will not process any s3 events that belong to other buckets")
		}

		if !(strings.HasSuffix(record.S3.Object.Key, "archive.zip")) {
			return errors.New("will not process any files that are not archive.zip")
		}

		ManageStreamingDownloadUpload(cfg, record.S3.Object.Key)
	}

	return nil
}
