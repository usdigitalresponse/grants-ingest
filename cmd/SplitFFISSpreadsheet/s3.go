package main

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3PutObjectAPI is the interface for writing new or replacement objects in an S3 bucket
type S3PutObjectAPI interface {
	// PutObject uploads an object to S3
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// UploadS3Object uploads bytes read from from r to an S3 object at the given bucket and key.
// If an error was encountered during upload, returns the error.
// Returns nil when the upload was successful.
func UploadS3Object(ctx context.Context, c S3PutObjectAPI, bucket, key string, r io.Reader) error {
	_, err := c.PutObject(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(bucket),
		Key:                  aws.String(key),
		Body:                 r,
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	})
	return err
}
