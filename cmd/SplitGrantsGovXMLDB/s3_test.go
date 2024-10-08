package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsTransport "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
)

type mockHeadObjectAPI func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)

func (m mockHeadObjectAPI) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return m(ctx, params, optFns...)
}

type mockPutObjectAPI func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)

func (m mockPutObjectAPI) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	return m(ctx, params, optFns...)
}

func createErrorResponseMap() map[int]*awsTransport.ResponseError {
	errorResponses := map[int]*awsTransport.ResponseError{}
	for _, statusCode := range []int{404, 500} {
		errorResponses[statusCode] = &awsTransport.ResponseError{
			ResponseError: &smithyhttp.ResponseError{Response: &smithyhttp.Response{
				Response: &http.Response{StatusCode: statusCode},
			}},
			RequestID: fmt.Sprintf("i-am-a-request-with-%d-status-response", statusCode),
		}
	}
	return errorResponses
}

func TestGetS3LastModified(t *testing.T) {
	now := time.Now()
	nilTime := func() *time.Time { return nil }()
	testBucketName := "test-bucket"
	testObjectKey := "test/key"
	errorResponses := createErrorResponseMap()

	for _, tt := range []struct {
		name            string
		client          func(t *testing.T) s3.HeadObjectAPIClient
		expLastModified *time.Time
		expErr          error
	}{
		{
			"Object exists",
			func(t *testing.T) s3.HeadObjectAPIClient {
				return mockHeadObjectAPI(func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
					t.Helper()
					assert.Equal(t, params.Bucket, aws.String(testBucketName))
					assert.Equal(t, params.Key, aws.String(testObjectKey))
					return &s3.HeadObjectOutput{LastModified: &now}, nil
				})
			},
			&now,
			nil,
		},
		{
			"Object does not exist",
			func(t *testing.T) s3.HeadObjectAPIClient {
				return mockHeadObjectAPI(func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
					t.Helper()
					assert.Equal(t, params.Bucket, aws.String(testBucketName))
					assert.Equal(t, params.Key, aws.String(testObjectKey))
					return &s3.HeadObjectOutput{}, fmt.Errorf("not found: %w", errorResponses[404])
				})
			},
			nilTime,
			nil,
		},
		{
			"Unexpected request failure",
			func(t *testing.T) s3.HeadObjectAPIClient {
				return mockHeadObjectAPI(func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
					t.Helper()
					assert.Equal(t, aws.String(testBucketName), params.Bucket)
					assert.Equal(t, aws.String(testObjectKey), params.Key)
					return &s3.HeadObjectOutput{}, fmt.Errorf("server error: %w", errorResponses[500])
				})
			},
			nilTime,
			errorResponses[500],
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			actualTime, actualErr := GetS3LastModified(
				context.TODO(), tt.client(t), testBucketName, testObjectKey)
			assert.Equal(t, tt.expLastModified, actualTime)
			if tt.expErr != nil {
				assert.ErrorAs(t, tt.expErr, &actualErr)
			}
		})
	}
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
