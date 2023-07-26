package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
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
		"ALLOWED_EMAIL_SENDERS":          "example.org",
		"GRANTS_SOURCE_DATA_BUCKET_NAME": "test-destination-bucket",
		"S3_USE_PATH_STYLE":              "true",
	}, &env)
	require.NoError(t, err, "Error configuring lambda environment for testing")
}

func setupS3ForTesting(t *testing.T, sourceBucket, destBucket string) *s3.Client {
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
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(sourceBucket)})
	require.NoError(t, err)
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(destBucket)})
	require.NoError(t, err)

	return client
}

func getFixture(t *testing.T, path string) *os.File {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err)
	return f
}

func TestHandleEvent(t *testing.T) {
	setupLambdaEnvForTesting(t)

	for _, tt := range []struct {
		name              string
		pathToFixture     string
		destinationBucket string
		uploadFixture     bool
		shouldError       bool
		errShouldContain  string
	}{
		{
			name:              "successful invocation",
			pathToFixture:     "fixtures/good.eml",
			destinationBucket: env.DestinationBucket,
			uploadFixture:     true,
			shouldError:       false,
		},
		{
			name:              "invalid email",
			pathToFixture:     "fixtures/bad_data.eml",
			destinationBucket: env.DestinationBucket,
			uploadFixture:     true,
			shouldError:       true,
			errShouldContain:  "failed to parse email from S3 object",
		},
		{
			name:              "object does not exist",
			pathToFixture:     "fixtures/good.eml",
			destinationBucket: env.DestinationBucket,
			uploadFixture:     false,
			shouldError:       true,
			errShouldContain:  "failed to retrieve S3 object",
		},
		{
			name:              "untrusted sender",
			pathToFixture:     "fixtures/bad_sender.eml",
			destinationBucket: env.DestinationBucket,
			uploadFixture:     true,
			shouldError:       true,
			errShouldContain:  "email cannot be trusted",
		},
		{
			name:              "copy object failure",
			pathToFixture:     "fixtures/good.eml",
			destinationBucket: "bucket-that-does-not-exist",
			uploadFixture:     true,
			shouldError:       true,
			errShouldContain:  "failed to copy S3 object",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sourceBucket := "source-bucket"
			sourceKey := "source/key.eml"
			svc := setupS3ForTesting(t, sourceBucket, tt.destinationBucket)
			if tt.uploadFixture {
				_, err := svc.PutObject(context.Background(), &s3.PutObjectInput{
					Bucket: aws.String(sourceBucket),
					Key:    aws.String(sourceKey),
					Body:   getFixture(t, tt.pathToFixture),
				})
				require.NoError(t, err)
			}

			err := handleEvent(context.Background(), svc, events.S3Event{
				Records: []events.S3EventRecord{{S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: sourceBucket},
					Object: events.S3Object{Key: sourceKey},
				}}},
			})

			if !tt.shouldError {
				assert.NoError(t, err)
				_, err := svc.HeadObject(context.Background(), &s3.HeadObjectInput{
					Bucket: aws.String(tt.destinationBucket),
					Key:    aws.String("sources/2023/04/22/ffis.org/raw.eml"),
				})
				assert.NoError(t, err, "Could not find the copied destination S3 object")
			} else {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.errShouldContain)
			}
		})
	}
}
