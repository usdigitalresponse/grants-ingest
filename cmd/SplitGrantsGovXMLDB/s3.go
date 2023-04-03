package main

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsTransport "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3GetObjectAPI is the interface for retrieving objects from an S3 bucket
type S3GetObjectAPI interface {
	// GetObject retrieves an object from S3
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// S3ReadObjectAPI is the interface for reading object contents and metadata from an S3 bucket
type S3ReadObjectAPI interface {
	S3GetObjectAPI
	s3.HeadObjectAPIClient
}

// S3PutObjectAPI is the interface for writing new or replacement objects in an S3 bucket
type S3PutObjectAPI interface {
	// PutObject uploads an object to S3
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3ReadWriteObjectAPI is the interface for reading to and writing from an S3 bucket
type S3ReadWriteObjectAPI interface {
	S3ReadObjectAPI
	S3PutObjectAPI
}

// GetS3LastModified gets the "Last Modified" time for the S3 object.
// If the object exists, a pointer to the last modification time is returned along with a nil error.
// If the specified object does not exist, the returned *time.Time and error are both nil.
// If an error is encountered when calling the HeadObject S3 API method, this will return a nil
// *time.Time value along with the encountered error.
func GetS3LastModified(ctx context.Context, c s3.HeadObjectAPIClient, bucket, key string) (*time.Time, error) {
	headOutput, err := c.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key)})
	if err != nil {
		var respError *awsTransport.ResponseError
		if errors.As(err, &respError) && respError.ResponseError.HTTPStatusCode() == 404 {
			return nil, nil
		}
		return nil, err
	}
	return headOutput.LastModified, nil
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
