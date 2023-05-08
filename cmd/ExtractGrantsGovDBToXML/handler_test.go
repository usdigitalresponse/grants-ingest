package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	goenv "github.com/Netflix/go-env"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
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
		"GRANTS_SOURCE_DATA_BUCKET_NAME": "test-destination-bucket",
		"S3_USE_PATH_STYLE":              "true",
		"MAX_DOWNLOAD_BACKOFF":           "1us",
	}, &env)
	require.NoError(t, err, "Error configuring lambda environment for testing")
}

func setupS3ForTesting(t *testing.T, sourceBucketName string) (*s3.Client, aws.Config, error) {
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
	bucketsToCreate := []string{sourceBucketName, env.GrantsDataBucket}
	for _, bucketName := range bucketsToCreate {
		fmt.Println(bucketName)
		_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucketName)})
		if err != nil {
			return client, cfg, err
		}
	}
	return client, cfg, nil
}

func TestLambdaInvocationScenarios(t *testing.T) {
	setupLambdaEnvForTesting(t)

	t.Run("Wrong source bucket raises an error", func(t *testing.T) {
		_, cfg, err := setupS3ForTesting(t, "source-bucket")
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err = handleS3EventWithConfig(cfg, ctx, events.S3Event{
			Records: []events.S3EventRecord{
				{S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: "not-source-bucket"},
					Object: events.S3Object{Key: "does/not/matter"},
				}},
			},
		})
		require.Error(t, err)
		assert.Equal(t, errors.New("will not process any s3 events that belong to other buckets"), err)
	})

	t.Run("Wrong file ending raises an error", func(t *testing.T) {
		_, cfg, err := setupS3ForTesting(t, "source-bucket")
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err = handleS3EventWithConfig(cfg, ctx, events.S3Event{
			Records: []events.S3EventRecord{
				{S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: "test-destination-bucket"},
					Object: events.S3Object{Key: "source/2023/01/01/invalid.zip"},
				}},
			},
		})
		require.Error(t, err)
		assert.Equal(t, errors.New("will not process any files that are not archive.zip"), err)
	})
}
