package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	goenv "github.com/Netflix/go-env"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-kit/log"
	"github.com/hashicorp/go-multierror"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/ffis"
)

func TestOpportunityS3ObjectKey(t *testing.T) {
	opp := opportunity{
		GrantID: 123456,
	}
	assert.Equal(t, opp.S3ObjectKey(), "123/123456/ffis.org/v1.json")
}

func setupLambdaEnvForTesting(t *testing.T) {
	t.Helper()

	// Suppress normal lambda log output
	logger = log.NewNopLogger()

	// Configure environment variables
	goenv.Unmarshal(goenv.EnvSet{
		"GRANTS_PREPARED_DATA_BUCKET_NAME": "test-destination-bucket",
		"S3_USE_PATH_STYLE":                "true",
		"DOWNLOAD_CHUNK_LIMIT":             "10",
	}, &env)
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
	bucketsToCreate := []string{sourceBucketName, env.DestinationBucket}
	for _, bucketName := range bucketsToCreate {
		_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucketName)})
		if err != nil {
			return client, cfg, err
		}
	}
	return client, cfg, nil
}

func TestLambaInvocation(t *testing.T) {
	setupLambdaEnvForTesting(t)

	sourceBucketName := "test-source-bucket"
	now := time.Now()
	s3client, cfg, err := setupS3ForTesting(t, sourceBucketName)
	assert.NoError(t, err, "Error configuring test environment")

	excelFixture, err := os.Open("fixtures/example_spreadsheet.xlsx")
	assert.NoError(t, err, "Error opening spreadsheet fixture")

	expectedOpp := ffis.FFISFundingOpportunity{
		CFDA:             "81.086",
		OppTitle:         "Example Opportunity 1",
		Agency:           "Office of Energy Efficiency and Renewable Energy",
		EstimatedFunding: 5000000,
		ExpectedAwards:   "N/A",
		OppNumber:        "ABC-0003065",
		GrantID:          123456,
		Eligibility: ffis.FFISFundingEligibility{
			State:           false,
			Local:           false,
			Tribal:          false,
			HigherEducation: false,
			NonProfits:      true,
			Other:           false,
		},
		Match: false,
		Bill:  "Infrastructure Investment and Jobs Act",
	}
	date, _ := time.Parse("1/2/2006", "5/11/2023")
	expectedOpp.DueDate = date

	t.Run("valid excel file", func(t *testing.T) {
		// Upload the test fixture to S3
		objectKey := fmt.Sprintf("sources/%s/ffis.org/download.xlsx", now.Format("2006/01/02"))
		_, err := s3client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(sourceBucketName),
			Key:    aws.String(objectKey),
			Body:   excelFixture,
		})
		require.NoErrorf(t, err, "Error creating test source object %s", objectKey)

		invocationErr := handleS3EventWithConfig(cfg, context.TODO(), events.S3Event{
			Records: []events.S3EventRecord{{
				S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: sourceBucketName},
					Object: events.S3Object{Key: objectKey},
				},
			}},
		})
		require.NoError(t, invocationErr)

		key := fmt.Sprintf("%s/%d/ffis.org/v1.json", strconv.FormatInt(expectedOpp.GrantID, 10)[:3], expectedOpp.GrantID)

		resp, err := s3client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(env.DestinationBucket),
			Key:    aws.String(key),
		})

		require.NoError(t, err)
		b, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		var savedOpportunity ffis.FFISFundingOpportunity
		assert.NoError(t, json.Unmarshal(b, &savedOpportunity))
		assert.Equal(t, expectedOpp, savedOpportunity)
	})

	t.Run("invalid excel file", func(t *testing.T) {
		setupLambdaEnvForTesting(t)

		sourceBucketName := "test-source-bucket"
		s3client, cfg, err := setupS3ForTesting(t, sourceBucketName)
		require.NoError(t, err)

		_, err = s3client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(sourceBucketName),
			Key:    aws.String("sources/2023/05/15/ffis.org/download.xlsx"),
			Body:   bytes.NewReader([]byte("foobar")),
		})
		require.NoError(t, err)

		err = handleS3EventWithConfig(cfg, context.TODO(), events.S3Event{
			Records: []events.S3EventRecord{
				{S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: sourceBucketName},
					Object: events.S3Object{Key: "sources/2023/05/15/ffis.org/download.xlsx"},
				}},
			},
		})
		require.Error(t, err)
		if errs, ok := err.(*multierror.Error); ok {
			assert.Equalf(t, 1, errs.Len(),
				"Invocation accumulated an unexpected number of errors: %s", errs)
		} else {
			require.Fail(t, "Invocation error could not be interpreted as *multierror.Error")
		}
	})

	t.Run("Context canceled during invocation", func(t *testing.T) {
		setupLambdaEnvForTesting(t)
		_, cfg, err := setupS3ForTesting(t, "source-bucket")
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err = handleS3EventWithConfig(cfg, ctx, events.S3Event{
			Records: []events.S3EventRecord{
				{S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: "source-bucket"},
					Object: events.S3Object{Key: "does/not/matter"},
				}},
			},
		})
		require.Error(t, err)
		if errs, ok := err.(*multierror.Error); ok {
			for _, wrapped := range errs.WrappedErrors() {
				assert.ErrorIs(t, wrapped, context.Canceled,
					"context.Canceled missing in accumulated error's chain")
			}
		} else {
			require.Fail(t, "Invocation error could not be interpreted as *multierror.Error")
		}
	})
}
