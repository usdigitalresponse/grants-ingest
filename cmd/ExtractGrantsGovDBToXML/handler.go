package main

import (
	"archive/zip"
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

const (
	MB                         = int64(1024 * 1024)
	GRANT_OPPORTUNITY_XML_NAME = "OpportunitySynopsisDetail_1_0"
)

type PipeWriter struct {
	io.Writer
}

type NewDataReader struct {
	*zipstream.Reader
}

type UploadResult struct {
	extractedFilekey *string
	err              error
}

func (r NewDataReader) NextR() (*zip.FileHeader, error) {
	fmt.Println("           Reading")
	return r.Next()
}

func (w PipeWriter) WriteAt(p []byte, offset int64) (n int, err error) {
	fmt.Println("Writing", offset)
	return w.Write(p)
}

func FileUploadStream(ctx context.Context, uploader *manager.Uploader, wg *sync.WaitGroup, pipeReader *io.PipeReader, objectKey string, uploadResult chan<- UploadResult) {
	data := zipstream.NewReader(pipeReader)
	defer wg.Done()

	new_data := NewDataReader{data}

	header, data_err := new_data.NextR()

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
		Body:   new_data,
	})

	if err != nil {
		uploadResult <- UploadResult{aws.String(""), fmt.Errorf("failed to upload extracted XML to S3 %v", err)}
		return
	}

	secondHeader, secondDataErr := new_data.NextR()
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

func FileDownloadStream(ctx context.Context, downloader *manager.Downloader, wg *sync.WaitGroup, pipeWriter *io.PipeWriter, objectKey string) {
	defer pipeWriter.Close()
	// defer wg.Done()

	_, err := downloader.Download(ctx, PipeWriter{pipeWriter}, &s3.GetObjectInput{
		Bucket: aws.String(env.GrantsDataBucket),
		Key:    aws.String(objectKey),
	})

	if err != nil {
		panic(err)
	}

	fmt.Println("Downloader is done.")
	wg.Done()
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
	wg.Add(2)
	uploadResult := make(chan UploadResult)
	go FileUploadStream(context.TODO(), uploader, &wg, pipeReader, objectKey, uploadResult)
	FileDownloadStream(context.TODO(), downloader, &wg, pipeWriter, objectKey)
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

func handleS3EventWithConfig(cfg aws.Config, ctx context.Context, s3Event events.S3Event) error {

	for _, record := range s3Event.Records {
		fmt.Println(record)
		logger := log.With(logger)
		log.Info(logger, "Following record was identified")
		log.Info(logger, record)

		if record.S3.Bucket.Name != env.GrantsDataBucket {
			return errors.New("will not process any s3 events that belong to other buckets")
			//log.Info(logger, "Will not process any s3 events that belong to other buckets")
			// continue
		}

		if !(strings.HasSuffix(record.S3.Object.Key, "archive.zip")) {
			return errors.New("will not process any files that are not archive.zip")
			// log.Info(logger, " Will not process any files that are not archive.zip")
			// continue
		}

		ManageStreamingDownloadUpload(cfg, record.S3.Object.Key)
	}

	return nil
}
