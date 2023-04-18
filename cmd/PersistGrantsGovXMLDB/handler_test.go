package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"
	"time"

	goenv "github.com/Netflix/go-env"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-kit/log"
	"github.com/hashicorp/go-multierror"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grantsgov "github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/grants.gov"
)

func setupLambdaEnvForTesting(t *testing.T) {
	t.Helper()

	// Suppress normal lambda log output
	logger = log.NewNopLogger()

	// Configure environment variables
	err := goenv.Unmarshal(goenv.EnvSet{
		"GRANTS_PREPARED_DYNAMODB_NAME": "test-destination-table",
		"S3_USE_PATH_STYLE":             "true",
	}, &env)
	require.NoError(t, err, "Error configuring environment variables for testing")
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
	bucketsToCreate := []string{sourceBucketName}
	for _, bucketName := range bucketsToCreate {
		_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucketName)})
		if err != nil {
			return client, cfg, err
		}
	}
	return client, cfg, nil
}

// func setupDynamoDBForTesting(t *testing.T, cfg aws.Config, tableName string) error {
// 	t.Helper()

// 	client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {})
// 	ctx := context.TODO()
// 	tablesToCreate := []string{tableName}
// 	for _, tableName := range tablesToCreate {
// 		_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
// 			TableName: aws.String(tableName),
// 			AttributeDefinitions: []types.AttributeDefinition{{
// 				AttributeName: aws.String("grant_id"),
// 				AttributeType: types.ScalarAttributeTypeS,
// 			}},
// 			KeySchema: []types.KeySchemaElement{{
// 				AttributeName: aws.String("grant_id"),
// 				KeyType:       types.KeyTypeHash,
// 			}},
// 		})
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

const SOURCE_OPPORTUNITY_TEMPLATE = `
<OpportunitySynopsisDetail_1_0>
	<OpportunityID>{{.OpportunityID}}</OpportunityID>
	<OpportunityTitle>Fun Grant</OpportunityTitle>
	<OpportunityNumber>ABCD-1234</OpportunityNumber>
	<OpportunityCategory>Some Category</OpportunityCategory>
	<FundingInstrumentType>Clarinet</FundingInstrumentType>
	<CategoryOfFundingActivity>My Funding Category</CategoryOfFundingActivity>
	<CategoryExplanation>Meow meow meow</CategoryExplanation>
	<CFDANumbers>1234.567</CFDANumbers>
	<EligibleApplicants>25</EligibleApplicants>
	<AdditionalInformationOnEligibility>This is some additional information on eligibility.</AdditionalInformationOnEligibility>
	<AgencyCode>TEST-AC</AgencyCode>
	<AgencyName>Bureau of Testing</AgencyName>
	<PostDate>09082022</PostDate>
	<CloseDate>01022023</CloseDate>
	<LastUpdatedDate>{{.LastUpdatedDate}}</LastUpdatedDate>
	<AwardCeiling>600000</AwardCeiling>
	<AwardFloor>400000</AwardFloor>
	<EstimatedTotalProgramFunding>600000</EstimatedTotalProgramFunding>
	<ExpectedNumberOfAwards>10</ExpectedNumberOfAwards>
	<Description>Here is a description of the opportunity.</Description>
	<Version>Synopsis 2</Version>
	<CostSharingOrMatchingRequirement>No</CostSharingOrMatchingRequirement>
	<ArchiveDate>02012023</ArchiveDate>
	<GrantorContactEmail>test@example.gov</GrantorContactEmail>
	<GrantorContactEmailDescription>Inquiries</GrantorContactEmailDescription>
	<GrantorContactText>Tester Person, Bureau of Testing, Office of Stuff &amp;lt;br/&amp;gt;</GrantorContactText>
</OpportunitySynopsisDetail_1_0>
`

func TestLambdaInvocationScenarios(t *testing.T) {
	t.Run("Missing source object", func(t *testing.T) {
		setupLambdaEnvForTesting(t)

		sourceBucketName := "test-source-bucket"
		s3client, cfg, err := setupS3ForTesting(t, sourceBucketName)
		require.NoError(t, err)
		// destinationTableName := "test-destination-table"
		// err = setupDynamoDBForTesting(t, cfg, destinationTableName)
		// require.NoError(t, err)
		sourceTemplate := template.Must(
			template.New("xml").Delims("{{", "}}").Parse(SOURCE_OPPORTUNITY_TEMPLATE),
		)
		var sourceData bytes.Buffer
		require.NoError(t, err)
		require.NoError(t, sourceTemplate.Execute(&sourceData, map[string]string{
			"OpportunityID":   "123456",
			"LastUpdatedDate": "01022023",
		}))
		require.NoError(t, err)
		_, err = s3client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(sourceBucketName),
			Key:    aws.String("123/123456/grants.gov/v2.xml"),
			Body:   bytes.NewReader(sourceData.Bytes()),
		})
		require.NoError(t, err)
		err = handleS3EventWithConfig(cfg, context.TODO(), events.S3Event{
			Records: []events.S3EventRecord{
				{S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: sourceBucketName},
					Object: events.S3Object{Key: "does/not/exist"},
				}},
				{S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: sourceBucketName},
					Object: events.S3Object{Key: "123/123456/grants.gov/v2.xml"},
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

type MockReader struct {
	read func([]byte) (int, error)
}

func (r *MockReader) Read(p []byte) (int, error) {
	return r.read(p)
}

func TestReadOpportunities(t *testing.T) {
	t.Run("Context cancelled between reads", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.TODO())
		err := readOpportunities(ctx, &MockReader{func(p []byte) (int, error) {
			cancel()
			return int(copy(p, []byte("<Grants>"))), nil
		}}, make(chan<- opportunity, 10))
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestProcessOpportunity(t *testing.T) {
	now := time.Now()
	testOpportunity := opportunity{
		OpportunityID:   "123456",
		LastUpdatedDate: grantsgov.MMDDYYYYType(now.Format(grantsgov.TimeLayoutMMDDYYYYType)),
	}

	t.Run("Error uploading to DynamoDB", func(t *testing.T) {
		setupLambdaEnvForTesting(t)
		dynamodbClient := mockDynamoDBUpdateItemAPI{
			mockUpdateItemAPI(func(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
				t.Helper()
				return nil, fmt.Errorf("some UpdateItem error")
			}),
		}
		err := processOpportunity(context.TODO(), dynamodbClient, testOpportunity)
		assert.ErrorContains(t, err, "Error uploading prepared grant opportunity to DynamoDB")
	})
}
