package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	goenv "github.com/Netflix/go-env"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-kit/log"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupLambdaEnvForTesting(t *testing.T) {
	t.Helper()

	// Suppress normal lambda log output
	logger = log.NewNopLogger()

	// Configure environment variables
	err := goenv.Unmarshal(goenv.EnvSet{
		"S3_USE_PATH_STYLE":   "true",
		"TMP_KEY_PATH_PREFIX": "tmp",
		"DOWNLOAD_PART_SIZE":  "0",
	}, &env)
	require.NoError(t, err, "Error configuring lambda environment for testing")
}

func setupS3ForTesting(t *testing.T, bucketName string) (*s3.Client, aws.Config, error) {
	t.Helper()

	// Start the S3 mock server and shut it down when the test ends
	backend := s3mem.New()
	faker := gofakes3.New(backend)
	ts := httptest.NewServer(faker.Server())
	t.Cleanup(ts.Close)

	cfg, _ := config.LoadDefaultConfig(
		context.TODO(),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("TEST", "TEST", "TESTING"),
		),
		config.WithHTTPClient(&http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		}),
		config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(_, _ string, _ ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: ts.URL}, nil
			}),
		),
	)

	// Create an Amazon S3 v2 client, important to use o.UsePathStyle
	// alternatively change local DNS settings, e.g., in /etc/hosts
	// to support requests to http://<bucketname>.127.0.0.1:32947/...
	client := s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true })
	ctx := context.TODO()
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucketName)})
	return client, cfg, err
}

type archiveFile struct {
	name string
	body []byte
}

func memoryZip(t *testing.T, files []archiveFile) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	z := zip.NewWriter(buf)
	defer z.Close()
	var err error
	for _, f := range files {
		var w io.Writer
		w, err = z.Create(f.name)
		if err != nil {
			break
		}
		if _, err = w.Write(f.body); err != nil {
			break
		}
	}
	require.NoError(t, err)
	return buf
}

type MockUploadManager struct {
	Err        error
	SideEffect func()
	callCount  int
}

func (u *MockUploadManager) Upload(context.Context, *s3.PutObjectInput, ...func(*manager.Uploader)) (*manager.UploadOutput, error) {
	u.callCount++
	if u.SideEffect != nil {
		u.SideEffect()
	}
	return new(manager.UploadOutput), u.Err
}

func (u *MockUploadManager) CallCount() int {
	return u.callCount
}

func TestFileUploadStream(t *testing.T) {
	setupLambdaEnvForTesting(t)
	bucket := "test-bucket"
	destKey := "tmp/path/to/extract.xml"

	t.Run("archive with single XML file", func(t *testing.T) {
		s3svc, _, err := setupS3ForTesting(t, bucket)
		require.NoError(t, err)
		ctx := context.Background()
		expectedXML := []byte("who can tell if I'm XML?")
		zip := memoryZip(t, []archiveFile{{
			fmt.Sprintf("GrantsDBExtract%sv2.xml", time.Now().Format("20060102")),
			expectedXML,
		}})

		require.NoError(t,
			fileUploadStream(ctx, manager.NewUploader(s3svc), zip, bucket, destKey))
		resp, err := s3svc.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(destKey),
		})
		assert.NoError(t, err)
		actual, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, expectedXML, actual)
	})

	t.Run("archive with non-XML file", func(t *testing.T) {
		assert.EqualError(t,
			fileUploadStream(
				context.Background(),
				&MockUploadManager{},
				memoryZip(t, []archiveFile{{"NotAnXMLFile.txt", []byte("doesn't matter")}}),
				bucket,
				destKey),
			fmt.Sprintf("unexpected non-XML file in zip stream: %s", "NotAnXMLFile.txt"))
	})

	t.Run("empty archive", func(t *testing.T) {
		assert.EqualError(t,
			fileUploadStream(
				context.Background(),
				&MockUploadManager{},
				new(bytes.Buffer),
				bucket,
				destKey),
			"error advancing to first entry in zip stream: EOF")
	})

	t.Run("invalid zip archive", func(t *testing.T) {
		notZip := new(bytes.Buffer)
		notZip.Write([]byte("oh no I am corrupt"))
		assert.EqualError(t,
			fileUploadStream(context.Background(), &MockUploadManager{}, notZip, bucket, destKey),
			"error advancing to first entry in zip stream: zip: not a valid zip file")
	})

	t.Run("upload failure", func(t *testing.T) {
		uploadErr := fmt.Errorf("you shall not pass")
		actualErr := fileUploadStream(
			context.Background(),
			&MockUploadManager{Err: uploadErr},
			memoryZip(t, []archiveFile{{"some.xml", []byte("doesn't matter")}}),
			bucket,
			destKey)
		assert.EqualError(t,
			actualErr,
			fmt.Sprintf("error uploading extracted XML to S3: %s", uploadErr.Error()))
		assert.ErrorIs(t, errors.Unwrap(actualErr), uploadErr)
	})

	t.Run("context cancelled", func(t *testing.T) {
		t.Run("before start", func(t *testing.T) {
			uploader := &MockUploadManager{}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := fileUploadStream(ctx, uploader, new(bytes.Buffer), bucket, destKey)
			assert.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, uploader.callCount, 0,
				"Upload() called unexpectedly after context was cancelled")
		})

		t.Run("after upload", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			uploader := &MockUploadManager{SideEffect: cancel}
			err := fileUploadStream(
				ctx,
				uploader,
				memoryZip(t, []archiveFile{{"some.xml", []byte("doesn't matter")}}),
				bucket,
				destKey)
			assert.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, uploader.CallCount(), 1, "Upload() unexpectedly not called")
		})
	})

	t.Run("additional file", func(t *testing.T) {
		err := fileUploadStream(
			context.Background(),
			&MockUploadManager{},
			memoryZip(t, []archiveFile{
				{"some.xml", []byte("doesn't matter")},
				{"the_spanish_inquisition.txt", []byte("nobody expects it")},
			}),
			bucket,
			destKey,
		)
		assert.EqualError(t, err,
			"unexpected additional file in zip archive: the_spanish_inquisition.txt")
	})
}

func TestFileDownloadStream(t *testing.T) {
	setupLambdaEnvForTesting(t)

	t.Run("context cancelled", func(t *testing.T) {
		bucket := "test-bucket"
		key := "does/not/matter"
		s3svc, _, err := setupS3ForTesting(t, bucket)
		require.NoError(t, err)
		s3svc.PutObject(context.Background(), &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   bytes.NewBuffer([]byte("some test data")),
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		assert.ErrorIs(t,
			fileDownloadStream(ctx, NewSequentialDownloadManager(s3svc), io.Discard, bucket, key),
			context.Canceled)
	})
}

type MockS3MoverAPIClient struct {
	Copier  func(context.Context, *s3.CopyObjectInput, ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
	Deleter func(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	// callCounter map[string]int
	callCounter *MockS3MoverAPIClientCallCounter
}

type MockS3MoverAPIClientCallCounter struct {
	CopyObject, DeleteObject int
}

func (c MockS3MoverAPIClient) CopyObject(
	ctx context.Context, input *s3.CopyObjectInput, opts ...func(*s3.Options),
) (*s3.CopyObjectOutput, error) {
	// c.callCounter["CopyObject"] += 1
	c.callCounter.CopyObject++
	return c.Copier(ctx, input, opts...)
}

func (c MockS3MoverAPIClient) DeleteObject(
	ctx context.Context, input *s3.DeleteObjectInput, opts ...func(*s3.Options),
) (*s3.DeleteObjectOutput, error) {
	c.callCounter.DeleteObject++
	return c.Deleter(ctx, input, opts...)
}

func TestMoveS3Object(t *testing.T) {
	setupLambdaEnvForTesting(t)
	bucket := "test-bucket"
	oldKey := "old"
	newKey := "new"
	prepareS3 := func(t *testing.T) *s3.Client {
		t.Helper()
		s3svc, _, err := setupS3ForTesting(t, bucket)
		require.NoError(t, err)
		_, err = manager.NewUploader(s3svc).Upload(context.Background(), &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(oldKey),
			Body:   bytes.NewBuffer([]byte("some data")),
		})
		require.NoError(t, err)
		return s3svc
	}

	t.Run("copy failure", func(t *testing.T) {
		s3svc := prepareS3(t)
		svc := MockS3MoverAPIClient{
			Copier: func(context.Context, *s3.CopyObjectInput, ...func(*s3.Options)) (
				*s3.CopyObjectOutput, error,
			) {
				return &s3.CopyObjectOutput{}, fmt.Errorf("some copy failure")
			},
			Deleter:     s3svc.DeleteObject,
			callCounter: &MockS3MoverAPIClientCallCounter{},
		}
		assert.EqualError(t,
			moveS3Object(context.Background(), svc, bucket, oldKey, newKey),
			"error copying extracted XML to permanent destination: some copy failure")
		assert.Equal(t, 1, svc.callCounter.CopyObject)
		assert.Equal(t, 0, svc.callCounter.DeleteObject)
	})

	t.Run("delete failure", func(t *testing.T) {
		s3svc := prepareS3(t)
		svc := MockS3MoverAPIClient{
			Copier: s3svc.CopyObject,
			Deleter: func(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
				return &s3.DeleteObjectOutput{}, fmt.Errorf("some delete failure")
			},
			callCounter: &MockS3MoverAPIClientCallCounter{},
		}
		assert.EqualError(t,
			moveS3Object(context.Background(), svc, bucket, oldKey, newKey),
			"error deleting extracted XML from temporary destination: some delete failure")
		assert.Equal(t, 1, svc.callCounter.CopyObject)
		assert.Equal(t, 1, svc.callCounter.DeleteObject)
	})
}

func TestHandleS3Event(t *testing.T) {
	setupLambdaEnvForTesting(t)
	bucket := "test-bucket"
	sourceKey := "path/to/archive.zip"
	destKey := "path/to/extract.xml"

	t.Run("valid zip file", func(t *testing.T) {
		s3svc, _, err := setupS3ForTesting(t, bucket)
		require.NoError(t, err)

		xmlContent := []byte("<xml>content</xml>")
		uploader := manager.NewUploader(s3svc)
		_, err = uploader.Upload(context.Background(), &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(sourceKey),
			Body:   memoryZip(t, []archiveFile{{"some.xml", xmlContent}}),
		})
		require.NoError(t, err)

		err = handleS3Event(context.Background(), s3svc, events.S3Event{
			Records: []events.S3EventRecord{{
				S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: bucket},
					Object: events.S3Object{Key: sourceKey},
				},
			}},
		})
		assert.NoError(t, err)
		resp, err := s3svc.GetObject(context.Background(), &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(destKey),
		})
		assert.NoError(t, err)
		unzipped := new(bytes.Buffer)
		_, err = io.Copy(unzipped, resp.Body)
		require.NoError(t, err)
		assert.Equal(t, xmlContent, unzipped.Bytes())
	})

	t.Run("extraction failure cancels download", func(t *testing.T) {
		s3svc, _, err := setupS3ForTesting(t, bucket)
		require.NoError(t, err)
		_, err = manager.NewUploader(s3svc).Upload(context.Background(), &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(sourceKey),
			Body:   bytes.NewBuffer(make([]byte, 100)),
		})
		require.NoError(t, err)

		// Ensure the downloader doesn't download everything at once (simulates a large download)
		env.DownloadPartSize = 1
		err = handleS3Event(context.Background(), s3svc, events.S3Event{
			Records: []events.S3EventRecord{{
				S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: bucket},
					Object: events.S3Object{Key: sourceKey},
				},
			}},
		})
		assert.ErrorIs(t, err, context.Canceled)
		assert.EqualError(t, err,
			"failed to stream zip archive to XML object: error streaming source object from S3: operation error S3: GetObject, context canceled")
	})
}
