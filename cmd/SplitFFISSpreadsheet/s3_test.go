package main

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
)

type mockPutObjectAPI func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)

func (m mockPutObjectAPI) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	return m(ctx, params, optFns...)
}

func TestUploadS3Object(t *testing.T) {
	testBucketName := "test-bucket"
	testObjectKey := "test/key"
	testReader := bytes.NewReader([]byte("hello!"))
	testError := fmt.Errorf("oh no this is an error")

	for _, tt := range []struct {
		name   string
		client func(t *testing.T) S3PutObjectAPI
		expErr error
	}{
		{
			"PutObject successful",
			func(t *testing.T) S3PutObjectAPI {
				return mockPutObjectAPI(func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					t.Helper()
					assert.Equal(t, aws.String(testBucketName), params.Bucket)
					assert.Equal(t, aws.String(testObjectKey), params.Key)
					assert.Equal(t, testReader, params.Body)
					assert.Equal(t, params.ServerSideEncryption, types.ServerSideEncryptionAes256)
					return &s3.PutObjectOutput{}, nil
				})
			},
			nil,
		},
		{
			"PutObject returns error",
			func(t *testing.T) S3PutObjectAPI {
				return mockPutObjectAPI(func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					t.Helper()
					assert.Equal(t, aws.String(testBucketName), params.Bucket)
					assert.Equal(t, aws.String(testObjectKey), params.Key)
					return &s3.PutObjectOutput{}, testError
				})
			},
			nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := UploadS3Object(context.TODO(), tt.client(t),
				testBucketName, testObjectKey, testReader)
			if tt.expErr != nil {
				assert.EqualError(t, err, tt.expErr.Error())
			}
		})
	}
}
