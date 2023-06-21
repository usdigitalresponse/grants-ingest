package main

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type (
	UploadManager interface {
		Upload(context.Context, *s3.PutObjectInput, ...func(*manager.Uploader)) (*manager.UploadOutput, error)
	}
	DownloadManager interface {
		Download(context.Context, io.WriterAt, *s3.GetObjectInput, ...func(*manager.Downloader)) (int64, error)
	}

	// S3UploaderDownloader is an API client that downloads and uploads S3 objects
	S3UploaderDownloaderAPIClient interface {
		manager.UploadAPIClient
		manager.DownloadAPIClient
	}

	// S3MoverAPIClient is an API client copies and (subsequently) deletes S3 objects
	S3MoverAPIClient interface {
		CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
		DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	}

	// S3UploaderDownloaderMoverAPIClient is an API client that downloads, uploads, and moves S3 objects
	S3UploaderDownloaderMoverAPIClient interface {
		S3UploaderDownloaderAPIClient
		S3MoverAPIClient
	}
)

// DummyWriterAt is an io.Writer that implements a dummy WriteAt method that just Writes.
// It is provided in order to satisfy the S3 uploader manager; since zip files are downloaded
// and extracted sequentially, we don't require support for writing at arbitrary offsets.
type DummyWriterAt struct {
	io.Writer
}

func (w DummyWriterAt) WriteAt(p []byte, offset int64) (n int, err error) {
	return w.Write(p)
}

func NewSequentialDownloadManager(c manager.DownloadAPIClient) DownloadManager {
	return manager.NewDownloader(c, func(d *manager.Downloader) {
		// Set concurrency to 1 so that content is downloaded sequentially for streaming unzip
		d.Concurrency = 1
		d.PartSize = env.DownloadPartSize
	})
}
