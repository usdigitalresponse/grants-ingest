package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"text/template"
	"time"

	goenv "github.com/Netflix/go-env"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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

func setupS3ForTesting(t *testing.T, sourceBucketName string) (*s3.Client, error) {
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
			return client, err
		}
	}
	return client, nil
}

const SOURCE_OPPORTUNITY_TEMPLATE = `
<OpportunitySynopsisDetail_1_0>
	<OpportunityID>{{.OpportunityID}}</OpportunityID>
	<OpportunityTitle>Fun Grant</OpportunityTitle>
	<OpportunityNumber>ABCD-1234</OpportunityNumber>
	<OpportunityCategory>Some Category</OpportunityCategory>
	<FundingInstrumentType>CA</FundingInstrumentType>
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

const SOURCE_FORECAST_TEMPLATE = `
<OpportunityForecastDetail_1_0>
	<OpportunityID>{{.OpportunityID}}</OpportunityID>
	<OpportunityTitle>Fun Grant</OpportunityTitle>
	<OpportunityNumber>ABCD-1234</OpportunityNumber>
	<OpportunityCategory>D</OpportunityCategory>
	<FundingInstrumentType>CA</FundingInstrumentType>
	<CategoryOfFundingActivity>My Funding Category</CategoryOfFundingActivity>
	<CFDANumbers>93.322</CFDANumbers>
	<EligibleApplicants>12</EligibleApplicants>
	<EligibleApplicants>13</EligibleApplicants>
	<EligibleApplicants>22</EligibleApplicants>
	<AdditionalInformationOnEligibility>This is some additional information on eligibility.</AdditionalInformationOnEligibility>
	<AgencyCode>TEST-AC</AgencyCode>
	<AgencyName>Bureau of Testing</AgencyName>
	<PostDate>09082029</PostDate>
	<LastUpdatedDate>{{.LastUpdatedDate}}</LastUpdatedDate>
	<EstimatedSynopsisPostDate>09082029</EstimatedSynopsisPostDate>
	<FiscalYear>2029</FiscalYear>
	<EstimatedSynopsisCloseDate>09082030</EstimatedSynopsisCloseDate>
	<EstimatedSynopsisCloseDateExplanation>Electronically submitted applications must be submitted no later than 11:59 p.m., ET, on the listed application due date.</EstimatedSynopsisCloseDateExplanation>
	<EstimatedAwardDate>09082035</EstimatedAwardDate>
	<EstimatedProjectStartDate>09082031</EstimatedProjectStartDate>
	<AwardCeiling>0</AwardCeiling>
	<AwardFloor>0</AwardFloor>
	<EstimatedTotalProgramFunding>60000</EstimatedTotalProgramFunding>
	<ExpectedNumberOfAwards>10</ExpectedNumberOfAwards>
	<Description>Here is a description.</Description>
	<Version>Forecast 1</Version>
	<CostSharingOrMatchingRequirement>No</CostSharingOrMatchingRequirement>
	<ArchiveDate>09092030</ArchiveDate>
	<GrantorContactEmail>test@example.gov</GrantorContactEmail>
	<GrantorContactEmailDescription>Inquiries</GrantorContactEmailDescription>
	<GrantorContactName>Tester Person, Bureau of Testing, Office of Stuff &amp;lt;br/&amp;gt;</GrantorContactName>
	<GrantorContactPhoneNumber>(555) 555-1234</GrantorContactPhoneNumber>
</OpportunityForecastDetail_1_0>
`

var (
	opportunityTemplate = template.Must(
		template.New("xml").Delims("{{", "}}").Parse(SOURCE_OPPORTUNITY_TEMPLATE))
	forecastTemplate = template.Must(
		template.New("xml").Delims("{{", "}}").Parse(SOURCE_FORECAST_TEMPLATE),
	)
)

func TestLambdaInvocationScenarios(t *testing.T) {
	t.Run("Missing source object", func(t *testing.T) {
		setupLambdaEnvForTesting(t)

		sourceBucketName := "test-source-bucket"
		s3Client, err := setupS3ForTesting(t, sourceBucketName)
		require.NoError(t, err)
		var sourceData bytes.Buffer
		require.NoError(t, err)
		require.NoError(t, opportunityTemplate.Execute(&sourceData, map[string]string{
			"OpportunityID":   "123456",
			"LastUpdatedDate": "01022023",
		}))
		require.NoError(t, err)
		_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(sourceBucketName),
			Key:    aws.String("123/123456/grants.gov/v2.xml"),
			Body:   bytes.NewReader(sourceData.Bytes()),
		})
		require.NoError(t, err)
		dynamodbClient := mockDynamoDBUpdateItemAPI{
			mockUpdateItemAPI(func(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
				t.Helper()
				return nil, nil
			}),
		}
		err = handleS3EventWithConfig(s3Client, dynamodbClient, context.TODO(), events.S3Event{
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

	t.Run("Empty XML stream", func(t *testing.T) {
		setupLambdaEnvForTesting(t)

		sourceBucketName := "test-source-bucket"
		s3Client, err := setupS3ForTesting(t, sourceBucketName)
		require.NoError(t, err)
		require.NoError(t, err)
		_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(sourceBucketName),
			Key:    aws.String("123/123456/grants.gov/v2.xml"),
			Body:   bytes.NewReader([]byte{}),
		})
		require.NoError(t, err)
		dynamodbClient := mockDynamoDBUpdateItemAPI{
			mockUpdateItemAPI(func(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
				t.Helper()
				require.Fail(t, "UpdateItem called unexpectedly")
				return nil, nil
			}),
		}
		err = handleS3EventWithConfig(s3Client, dynamodbClient, context.TODO(), events.S3Event{
			Records: []events.S3EventRecord{
				{S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: sourceBucketName},
					Object: events.S3Object{Key: "123/123456/grants.gov/v2.xml"},
				}},
			},
		})
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("Context canceled during invocation", func(t *testing.T) {
		setupLambdaEnvForTesting(t)
		s3Client, err := setupS3ForTesting(t, "source-bucket")
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		dynamodbClient := mockDynamoDBUpdateItemAPI{
			mockUpdateItemAPI(func(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
				t.Helper()
				return nil, nil
			}),
		}

		err = handleS3EventWithConfig(s3Client, dynamodbClient, ctx, events.S3Event{
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

	t.Run("Decodes and persists", func(t *testing.T) {
		for _, tt := range []struct {
			grantRecordType string
			xmlTemplate     *template.Template
		}{
			{"opportunity", opportunityTemplate},
			{"forecast", forecastTemplate},
		} {
			t.Run(tt.grantRecordType, func(t *testing.T) {

				setupLambdaEnvForTesting(t)

				sourceBucketName := "test-source-bucket"
				s3Client, err := setupS3ForTesting(t, sourceBucketName)
				require.NoError(t, err)
				var sourceData bytes.Buffer
				require.NoError(t, err)
				require.NoError(t, tt.xmlTemplate.Execute(&sourceData, map[string]string{
					"OpportunityID":   "123456",
					"LastUpdatedDate": "01022023",
				}))
				require.NoError(t, err)
				_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
					Bucket: aws.String(sourceBucketName),
					Key:    aws.String("123/123456/grants.gov/v2.xml"),
					Body:   bytes.NewReader(sourceData.Bytes()),
				})
				require.NoError(t, err)
				dynamodbClient := mockDynamoDBUpdateItemAPI{
					mockUpdateItemAPI(func(ctx context.Context, params *dynamodb.UpdateItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
						t.Helper()

						// Check expected item key
						var itemKey string
						assert.NoError(t, attributevalue.Unmarshal(params.Key["grant_id"], &itemKey))
						assert.Equal(t, "123456", itemKey)

						// Check expected `is_forecast` attribute value
						attrValuePlaceholder := findDDBExpressionAttributePlaceholder(t,
							"is_forecast", params.ExpressionAttributeNames)
						attrValue := params.ExpressionAttributeValues[attrValuePlaceholder]
						var itemIsForecast bool
						assert.NoError(t, attributevalue.Unmarshal(attrValue, &itemIsForecast))
						if tt.grantRecordType == "opportunity" {
							assert.False(t, itemIsForecast)
						} else if tt.grantRecordType == "forecast" {
							assert.True(t, itemIsForecast)
						} else {
							require.Fail(t, "Cannot test an unrecognized grantRecord type",
								"Expected grantRecord type %q or %q but received %q",
								"opportunity", "forecast", tt.grantRecordType)
						}
						return nil, nil
					}),
				}
				err = handleS3EventWithConfig(s3Client, dynamodbClient, context.TODO(), events.S3Event{
					Records: []events.S3EventRecord{{S3: events.S3Entity{
						Bucket: events.S3Bucket{Name: sourceBucketName},
						Object: events.S3Object{Key: "123/123456/grants.gov/v2.xml"},
					}}},
				})
				require.NoError(t, err)
			})
		}
	})
}

// DynamoDB maps attribute names and corresponding values to placeholders.
// e.g. `{"#13": "some_attribute_name"}` corresponds to `{":13": true}` means `some_attribute_name = true`.
// Note that the placeholder numbers are assigned in arbitrary order.
// This helper identifies the name placeholder that corresponds to the targetName attribute
// and converts it to a value placeholder by replacing `#` with `:`.
func findDDBExpressionAttributePlaceholder(t *testing.T, targetName string, expressionAttributeNames map[string]string) string {
	t.Helper()
	for namePlaceholder, attrName := range expressionAttributeNames {
		if attrName == targetName {
			return strings.Replace(namePlaceholder, "#", ":", 1)
		}
	}
	require.Failf(t, "Failed to locate target attribute in DynamoDB expression attribute names mapping",
		"Could not find %q in the following name placeholders map: %v",
		targetName, expressionAttributeNames)
	return ""
}

func TestDecodeNextGrantRecord(t *testing.T) {
	getOpportunityXML := func(OpportunityID, LastUpdatedDate string) *bytes.Buffer {
		t.Helper()
		data := &bytes.Buffer{}
		require.NoError(t, opportunityTemplate.Execute(data, map[string]string{
			"OpportunityID":   OpportunityID,
			"LastUpdatedDate": LastUpdatedDate,
		}), "Unexpected error generating opportunity XML data during test setup")
		return data
	}
	getForecastXML := func(OpportunityID, LastUpdatedDate string) *bytes.Buffer {
		t.Helper()
		data := &bytes.Buffer{}
		require.NoError(t, forecastTemplate.Execute(data, map[string]string{
			"OpportunityID":   OpportunityID,
			"LastUpdatedDate": LastUpdatedDate,
		}), "Unexpected error generating forecast XML data during test setup")
		return data
	}
	testDateString := time.Now().Format(grantsgov.TimeLayoutMMDDYYYYType)

	t.Run("Decodes opportunity grantRecord from XML", func(t *testing.T) {
		o, err := decodeNextGrantRecord(getOpportunityXML("12345", testDateString))
		assert.NoError(t, err)
		if assert.IsType(t, opportunity{}, o) {
			assert.Equal(t, "12345", string(o.(opportunity).OpportunityID),
				"opportunity has unexpected OpportunityID")
		}
	})

	t.Run("Decodes forecast grantRecord from XML", func(t *testing.T) {
		f, err := decodeNextGrantRecord(getForecastXML("12345", testDateString))
		assert.NoError(t, err)
		if assert.IsType(t, forecast{}, f) {
			assert.Equal(t, "12345", string(f.(forecast).OpportunityID),
				"forecast has unexpected OpportunityID")
		}
	})

	t.Run("Stops reading after decoding next grantRecord or EOF", func(t *testing.T) {
		r := io.MultiReader(
			getOpportunityXML("12345", testDateString),
			getForecastXML("56789", testDateString),
			getForecastXML("13579", testDateString),
			getOpportunityXML("24680", testDateString),
		)
		o1, err := decodeNextGrantRecord(r)
		assert.NoError(t, err)
		if assert.IsType(t, opportunity{}, o1) {
			assert.Equal(t, "12345", string(o1.(opportunity).OpportunityID),
				"opportunity has unexpected OpportunityID")
		}

		f1, err := decodeNextGrantRecord(r)
		assert.NoError(t, err)
		if assert.IsType(t, forecast{}, f1) {
			assert.Equal(t, "56789", string(f1.(forecast).OpportunityID),
				"forecast has unexpected OpportunityID")
		}

		f2, err := decodeNextGrantRecord(r)
		assert.NoError(t, err)
		if assert.IsType(t, forecast{}, f2) {
			assert.Equal(t, "13579", string(f2.(forecast).OpportunityID),
				"forecast has unexpected OpportunityID")
		}

		o2, err := decodeNextGrantRecord(r)
		assert.NoError(t, err)
		if assert.IsType(t, opportunity{}, o2) {
			assert.Equal(t, "24680", string(o2.(opportunity).OpportunityID),
				"opportunity has unexpected OpportunityID")
		}

		finalRV, err := decodeNextGrantRecord(r)
		assert.Nil(t, finalRV)
		assert.ErrorIs(t, err, io.EOF)
	})
}

func TestProcessGrantRecord(t *testing.T) {
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
		err := processGrantRecord(context.TODO(), dynamodbClient, testOpportunity)
		assert.ErrorContains(t, err, "Error uploading prepared grant opportunity to DynamoDB")
	})

	t.Run("Conditional check failed", func(t *testing.T) {
		setupLambdaEnvForTesting(t)
		dynamodbClient := mockDynamoDBUpdateItemAPI{
			mockUpdateItemAPI(func(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
				t.Helper()
				var err = &types.ConditionalCheckFailedException{
					Message: aws.String("The conditional request failed"),
				}
				return nil, err
			}),
		}
		assert.NoError(t, processGrantRecord(context.TODO(), dynamodbClient, testOpportunity))
	})
}
