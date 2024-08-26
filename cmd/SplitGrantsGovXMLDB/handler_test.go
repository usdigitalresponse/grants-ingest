package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"
	"time"

	goenv "github.com/Netflix/go-env"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsTransport "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/go-kit/log"
	"github.com/hashicorp/go-multierror"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grantsgov "github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/grants.gov"
)

type mockDDBGetItemRVLookup map[string]mockDDBGetItemRV

func (m mockDDBGetItemRVLookup) AddEntry(id, lastModified string, err error) {
	m[id] = mockDDBGetItemRV{lastModified, err}
}

type mockDDBGetItemRV struct {
	lastModified string
	err          error
}

func newMockDDBClient(t *testing.T, idToRV mockDDBGetItemRVLookup) mockDynamoDBGetItemClient {
	t.Helper()
	return mockDynamoDBGetItemClient(func(ctx context.Context, params *dynamodb.GetItemInput, f ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		key := struct{ grant_id string }{}
		err := attributevalue.UnmarshalMap(params.Key, &key)
		require.NoError(t, err, "Failed to extract grant_id value from DynamoDB GetItem key")
		output := dynamodb.GetItemOutput{Item: nil}
		if rv, exists := idToRV[key.grant_id]; exists {
			output.Item = map[string]types.AttributeValue{
				"LastUpdatedDate": &ddbtypes.AttributeValueMemberS{Value: rv.lastModified},
			}
			return &output, rv.err
		}
		return &output, nil
	})
}

func setupLambdaEnvForTesting(t *testing.T) {
	t.Helper()

	// Suppress normal lambda log output
	logger = log.NewNopLogger()

	// Configure environment variables
	goenv.Unmarshal(goenv.EnvSet{
		"GRANTS_PREPARED_DATA_BUCKET_NAME": "test-destination-bucket",
		"GRANTS_PREPARED_DATA_TABLE_NAME":  "test-dynamodb-table",
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

const SOURCE_FORECAST_TEMPLATE = `
<OpportunityForecastDetail_1_0>
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
	<EstimatedSynopsisPostDate>02102016</EstimatedSynopsisPostDate>
	<FiscalYear>2016</FiscalYear>
	<EstimatedSynopsisCloseDate>04112016</EstimatedSynopsisCloseDate>
	<EstimatedAwardDate>09082016</EstimatedAwardDate>
	<EstimatedProjectStartDate>09302016</EstimatedProjectStartDate>
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
	<GrantorContactName>Tester Person</GrantorContactName>
	<GrantorContactPhoneNumber>800-123-4567</GrantorContactPhoneNumber>
</OpportunityForecastDetail_1_0>
`

type grantValues struct {
	template        string
	OpportunityID   string
	LastUpdatedDate string
	isExtant        bool
	isValid         bool
	isSkipped       bool
	isForecast      bool
}

func (values grantValues) getFilename() string {
	if values.isForecast {
		return "v2.OpportunityForecastDetail_1_0.xml"
	} else {
		return "v2.OpportunitySynopsisDetail_1_0.xml"
	}
}

func TestLambdaInvocationScenarios(t *testing.T) {
	setupLambdaEnvForTesting(t)
	sourceBucketName := "test-source-bucket"
	now := time.Now()
	s3client, _, err := setupS3ForTesting(t, sourceBucketName)
	assert.NoError(t, err, "Error configuring test environment")

	seenOpportunityIDs := make(map[string]struct{})

	for _, tt := range []struct {
		name                      string
		isForecastedGrantsEnabled bool
		grantValues               []grantValues
	}{
		{
			"Well-formed source XML for single new grant",
			true,
			[]grantValues{
				{
					SOURCE_OPPORTUNITY_TEMPLATE,
					"1001",
					now.AddDate(-1, 0, 0).Format("01022006"),
					false,
					true,
					false,
					false,
				},
			},
		},
		{
			"Well-formed source XML for single new forecast",
			true,
			[]grantValues{
				{
					SOURCE_FORECAST_TEMPLATE,
					"1002",
					now.AddDate(-1, 0, 0).Format("01022006"),
					false,
					true,
					false,
					true,
				},
			},
		},
		{
			"When flag is disabled, ignores well-formed source XML for single new forecast",
			false,
			[]grantValues{
				{
					SOURCE_FORECAST_TEMPLATE,
					"1003",
					now.AddDate(-1, 0, 0).Format("01022006"),
					false,
					true,
					true,
					true,
				},
			},
		},
		{
			"Mixed well-formed grant and forecast",
			true,
			[]grantValues{
				{
					SOURCE_OPPORTUNITY_TEMPLATE,
					"1004",
					now.AddDate(-1, 0, 0).Format("01022006"),
					false,
					true,
					false,
					false,
				},
				{
					SOURCE_FORECAST_TEMPLATE,
					"1005",
					now.AddDate(-1, 0, 0).Format("01022006"),
					false,
					true,
					false,
					true,
				},
			},
		},
		{
			"One grant to update and one to ignore",
			true,
			[]grantValues{
				{
					SOURCE_OPPORTUNITY_TEMPLATE,
					"1006",
					now.AddDate(-1, 0, 0).Format("01022006"),
					true,
					true,
					false,
					false,
				},
				{
					SOURCE_OPPORTUNITY_TEMPLATE,
					"1007",
					now.AddDate(1, 0, 0).Format("01022006"),
					true,
					true,
					false,
					false,
				},
			},
		},
		{
			"One grant to update and one with malformed source data",
			true,
			[]grantValues{
				{
					SOURCE_OPPORTUNITY_TEMPLATE,
					"1008",
					now.AddDate(-1, 0, 0).Format("01022006"),
					true,
					true,
					false,
					false,
				},
				{
					`<OpportunitySynopsisDetail_1_0>
					<OpportunityID>{{.OpportunityID}}</OpportunityID>
					<LastUpdatedDate>{{.LastUpdatedDate}}</LastUpdatedDate>
					<OpportunityTitle>Fun Grant`,
					"1009",
					now.AddDate(-1, 0, 0).Format("01022006"),
					false,
					false,
					false,
					false,
				},
			},
		},
		{
			"One grant with invalid date format",
			true,
			[]grantValues{
				{
					SOURCE_OPPORTUNITY_TEMPLATE,
					"1010",
					now.AddDate(-1, 0, 0).Format("01/02/06"),
					false,
					false,
					false,
					false,
				},
			},
		},
		{
			"Source contains invalid token",
			true,
			[]grantValues{
				{
					"<invalidtoken",
					"1011",
					now.AddDate(-1, 0, 0).Format("01/02/06"),
					false,
					false,
					false,
					false,
				},
			},
		},
	} {
		env.IsForecastedGrantsEnabled = tt.isForecastedGrantsEnabled

		// Verify there are no previously seen grant IDs, as they can cause unexpected interactions in
		// our testing AWS setup
		for _, gv := range tt.grantValues {
			if _, exists := seenOpportunityIDs[gv.OpportunityID]; exists {
				t.Fatalf("Duplicate opportunity ID found: %s in test case '%s'", gv.OpportunityID, tt.name)
			}
			seenOpportunityIDs[gv.OpportunityID] = struct{}{}
		}

		// Build the source XML to test, based on the test case parameters
		// (will also place extant records in S3 if specified in the test case)
		var sourceGrantsData bytes.Buffer
		sourceOpportunitiesData := make(map[string][]byte)
		dynamodbLookups := make(mockDDBGetItemRVLookup)
		_, err := sourceGrantsData.WriteString("<Grants>")
		require.NoError(t, err)
		for _, values := range tt.grantValues {
			var sourceOpportunityData bytes.Buffer
			sourceTemplate := template.Must(
				template.New("xml").Delims("{{", "}}").Parse(values.template),
			)
			require.NoError(t, sourceTemplate.Execute(&sourceOpportunityData, map[string]string{
				"OpportunityID":   values.OpportunityID,
				"LastUpdatedDate": values.LastUpdatedDate,
			}))
			if values.isExtant {
				extantKey := fmt.Sprintf("%s/%s/grants.gov/%s",
					values.OpportunityID[0:3], values.OpportunityID, values.getFilename())
				_, err := s3client.PutObject(context.TODO(), &s3.PutObjectInput{
					Bucket: aws.String(env.DestinationBucket),
					Key:    aws.String(extantKey),
					Body:   bytes.NewReader(sourceOpportunityData.Bytes()),
				})
				require.NoError(t, err)
				dynamodbLookups[values.OpportunityID] = mockDDBGetItemRV{lastModified: values.LastUpdatedDate}
			}
			_, err = sourceGrantsData.Write(sourceOpportunityData.Bytes())
			require.NoError(t, err)
			sourceOpportunitiesData[values.OpportunityID] = sourceOpportunityData.Bytes()
		}
		_, err = sourceGrantsData.WriteString("</Grants>")
		require.NoError(t, err)

		// Execute the test case
		t.Run(tt.name, func(t *testing.T) {
			// Place the XML file constructed above into the correct S3 location
			objectKey := fmt.Sprintf("sources/%s/grants.gov/extract.xml", now.Format("2006/01/02"))
			_, err := s3client.PutObject(context.TODO(), &s3.PutObjectInput{
				Bucket: aws.String(sourceBucketName),
				Key:    aws.String(objectKey),
				Body:   bytes.NewReader(sourceGrantsData.Bytes()),
			})
			require.NoErrorf(t, err, "Error creating test source object %s", objectKey)

			// Invoke the handler under test with a constructed S3 event
			invocationErr := handleS3Event(context.TODO(), s3client, newMockDDBClient(t, dynamodbLookups), events.S3Event{
				Records: []events.S3EventRecord{{
					S3: events.S3Entity{
						Bucket: events.S3Bucket{Name: sourceBucketName},
						Object: events.S3Object{Key: objectKey},
					},
				}},
			})

			// Determine the list of expected grant objects to have been saved by the handler
			sourceContainsInvalidOpportunities := false
			for _, v := range tt.grantValues {
				if !v.isValid {
					sourceContainsInvalidOpportunities = true
				}
			}
			if sourceContainsInvalidOpportunities {
				require.Error(t, invocationErr)
			} else {
				require.NoError(t, invocationErr)
			}
			var expectedGrants grantsgov.Grants
			err = xml.Unmarshal(sourceGrantsData.Bytes(), &expectedGrants)
			if !sourceContainsInvalidOpportunities {
				require.NoError(t, err)
			}

			// For each grant value in the test case, we'll verify it was handled correctly
			for _, v := range tt.grantValues {
				key := fmt.Sprintf("%s/%s/grants.gov/%s",
					v.OpportunityID[0:3], v.OpportunityID, v.getFilename())
				resp, err := s3client.GetObject(context.TODO(), &s3.GetObjectInput{
					Bucket: aws.String(env.DestinationBucket),
					Key:    aws.String(key),
				})

				if v.isSkipped || (!v.isValid && !v.isExtant) {
					// If there was no extant file and the new grant is invalid,
					// or if we were meant to skip this grant,
					// then there should be no S3 file
					assert.Error(t, err)
				} else {
					// Otherwise, we verify the S3 file matches the source from the test case
					require.NoError(t, err)
					b, err := io.ReadAll(resp.Body)
					require.NoError(t, err)
					var sourceOpportunity, savedOpportunity grantsgov.OpportunitySynopsisDetail_1_0
					assert.NoError(t, xml.Unmarshal(b, &savedOpportunity))
					require.NoError(t, xml.Unmarshal(
						sourceOpportunitiesData[v.OpportunityID],
						&sourceOpportunity))
					assert.Equal(t, sourceOpportunity, savedOpportunity)
				}
			}
		})
	}

	t.Run("Missing source object", func(t *testing.T) {
		setupLambdaEnvForTesting(t)

		sourceBucketName := "test-source-bucket"
		s3client, _, err := setupS3ForTesting(t, sourceBucketName)
		require.NoError(t, err)
		sourceTemplate := template.Must(
			template.New("xml").Delims("{{", "}}").Parse(SOURCE_OPPORTUNITY_TEMPLATE),
		)
		var sourceData bytes.Buffer
		_, err = sourceData.WriteString("<Grants>")
		require.NoError(t, err)
		require.NoError(t, sourceTemplate.Execute(&sourceData, map[string]string{
			"OpportunityID":   "12345",
			"LastUpdatedDate": "01022023",
		}))
		_, err = sourceData.WriteString("</Grants>")
		require.NoError(t, err)
		_, err = s3client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(sourceBucketName),
			Key:    aws.String("sources/2023/02/03/grants.gov/extract.xml"),
			Body:   bytes.NewReader(sourceData.Bytes()),
		})
		require.NoError(t, err)
		err = handleS3Event(context.TODO(), s3client, newMockDDBClient(t, mockDDBGetItemRVLookup{}), events.S3Event{
			Records: []events.S3EventRecord{
				{S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: sourceBucketName},
					Object: events.S3Object{Key: "does/not/exist"},
				}},
				{S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: sourceBucketName},
					Object: events.S3Object{Key: "sources/2023/02/03/grants.gov/extract.xml"},
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

		_, err = s3client.GetObject(context.Background(), &s3.GetObjectInput{
			Bucket: aws.String(env.DestinationBucket),
			Key:    aws.String("123/12345/grants.gov/v2.OpportunitySynopsisDetail_1_0.xml"),
		})
		assert.NoError(t, err, "Expected destination object was not created")
	})

	t.Run("Context canceled during invocation", func(t *testing.T) {
		setupLambdaEnvForTesting(t)
		_, _, err := setupS3ForTesting(t, "source-bucket")
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err = handleS3Event(ctx, s3client, newMockDDBClient(t, mockDDBGetItemRVLookup{}), events.S3Event{
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

func TestReadRecords(t *testing.T) {
	t.Run("Context cancelled between reads", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.TODO())
		err := readRecords(ctx, &MockReader{func(p []byte) (int, error) {
			cancel()
			return int(copy(p, []byte("<Grants>"))), nil
		}}, make(chan<- grantRecord, 10))
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestProcessRecord(t *testing.T) {
	now := time.Now()
	testOpportunity := opportunity{
		OpportunityID:   "1234",
		LastUpdatedDate: grantsgov.MMDDYYYYType(now.Format(grantsgov.TimeLayoutMMDDYYYYType)),
	}

	t.Run("Error uploading to S3", func(t *testing.T) {
		setupLambdaEnvForTesting(t)
		s3Client := mockS3ReadwriteObjectAPI{
			mockHeadObjectAPI(
				func(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
					t.Helper()
					return nil, &awsTransport.ResponseError{
						ResponseError: &smithyhttp.ResponseError{Response: &smithyhttp.Response{
							Response: &http.Response{StatusCode: 404},
						}},
					}
				},
			),
			mockGetObjectAPI(func(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				t.Helper()
				require.Fail(t, "GetObject called unexpectedly")
				return nil, nil
			}),
			mockPutObjectAPI(func(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
				t.Helper()
				return nil, fmt.Errorf("some PutObject error")
			}),
		}
		fmt.Printf("%T", s3Client)
		err := processRecord(context.TODO(), s3Client, newMockDDBClient(t, map[string]mockDDBGetItemRV{
			string(testOpportunity.OpportunityID): {lastModified: string(testOpportunity.LastUpdatedDate), err: nil},
		}), testOpportunity)
		assert.ErrorContains(t, err, "Error uploading prepared grant record to S3")
	})
}
