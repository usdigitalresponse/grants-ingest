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
	s3client, cfg, err := setupS3ForTesting(t, sourceBucketName)
	assert.NoError(t, err, "Error configuring test environment")

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
					"1234",
					now.AddDate(-1, 0, 0).Format("01022006"),
					false,
					true,
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
					"2345",
					now.AddDate(-1, 0, 0).Format("01022006"),
					false,
					true,
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
					"2345",
					now.AddDate(-1, 0, 0).Format("01022006"),
					false,
					false,
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
					"1234",
					now.AddDate(-1, 0, 0).Format("01022006"),
					false,
					true,
					false,
				},
				{
					SOURCE_FORECAST_TEMPLATE,
					"2345",
					now.AddDate(-1, 0, 0).Format("01022006"),
					false,
					true,
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
					"2345",
					now.AddDate(-1, 0, 0).Format("01022006"),
					true,
					true,
					false,
				},
				{
					SOURCE_OPPORTUNITY_TEMPLATE,
					"3456",
					now.AddDate(1, 0, 0).Format("01022006"),
					true,
					true,
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
					"4567",
					now.AddDate(-1, 0, 0).Format("01022006"),
					true,
					true,
					false,
				},
				{
					`<OpportunitySynopsisDetail_1_0>
					<OpportunityID>{{.OpportunityID}}</OpportunityID>
					<LastUpdatedDate>{{.LastUpdatedDate}}</LastUpdatedDate>
					<OpportunityTitle>Fun Grant`,
					"5678",
					now.AddDate(-1, 0, 0).Format("01022006"),
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
					"6789",
					now.AddDate(-1, 0, 0).Format("01/02/06"),
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
					"7890",
					now.AddDate(-1, 0, 0).Format("01/02/06"),
					false,
					false,
					false,
				},
			},
		},
	} {
		// Configure forecasted flag in environment variables
		if tt.isForecastedGrantsEnabled {
			goenv.Unmarshal(goenv.EnvSet{
				"IS_FORECASTED_GRANTS_ENABLED": "true",
			}, &env)
		} else {
			goenv.Unmarshal(goenv.EnvSet{
				"IS_FORECASTED_GRANTS_ENABLED": "false",
			}, &env)
		}

		var sourceGrantsData bytes.Buffer
		sourceOpportunitiesData := make(map[string][]byte)
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
			}
			_, err = sourceGrantsData.Write(sourceOpportunityData.Bytes())
			require.NoError(t, err)
			sourceOpportunitiesData[values.OpportunityID] = sourceOpportunityData.Bytes()
		}
		_, err = sourceGrantsData.WriteString("</Grants>")
		require.NoError(t, err)

		t.Run(tt.name, func(t *testing.T) {
			objectKey := fmt.Sprintf("sources/%s/grants.gov/extract.xml", now.Format("2006/01/02"))
			_, err := s3client.PutObject(context.TODO(), &s3.PutObjectInput{
				Bucket: aws.String(sourceBucketName),
				Key:    aws.String(objectKey),
				Body:   bytes.NewReader(sourceGrantsData.Bytes()),
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

			for _, v := range tt.grantValues {
				key := fmt.Sprintf("%s/%s/grants.gov/%s",
					v.OpportunityID[0:3], v.OpportunityID, v.getFilename())
				resp, err := s3client.GetObject(context.TODO(), &s3.GetObjectInput{
					Bucket: aws.String(env.DestinationBucket),
					Key:    aws.String(key),
				})

				if !v.isValid && !v.isExtant {
					assert.Error(t, err)
				} else {
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
		s3client, cfg, err := setupS3ForTesting(t, sourceBucketName)
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
		err = handleS3EventWithConfig(cfg, context.TODO(), events.S3Event{
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

	t.Run("Destination bucket is incorrectly configured", func(t *testing.T) {
		setupLambdaEnvForTesting(t)
		c := mockS3ReadwriteObjectAPI{
			mockHeadObjectAPI(
				func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
					t.Helper()
					return &s3.HeadObjectOutput{}, fmt.Errorf("server error")
				},
			),
			mockGetObjectAPI(nil),
			mockPutObjectAPI(nil),
		}
		err := processRecord(context.TODO(), c, testOpportunity)
		assert.ErrorContains(t, err, "Error determining last modified time for remote record")
	})

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
		err := processRecord(context.TODO(), s3Client, testOpportunity)
		assert.ErrorContains(t, err, "Error uploading prepared grant record to S3")
	})
}
