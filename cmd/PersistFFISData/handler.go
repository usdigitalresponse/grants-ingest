package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	"github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/ffis"
)

type S3API interface {
	GetObject(ctx context.Context,
		params *s3.GetObjectInput,
		optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// error constants
var (
	ErrMissingBill    = fmt.Errorf("bill missing from FFIS data")
	ErrMissingGrantID = fmt.Errorf("grant id missing from FFIS data")
)

func handleS3Event(ctx context.Context, s3Event events.S3Event, s3client S3API, dbapi DynamoDBUpdateItemAPI) error {
	uploadedFile := s3Event.Records[0].S3.Object.Key
	bucket := s3Event.Records[0].S3.Bucket.Name
	logger := log.With(logger, "source_key", uploadedFile, "source_bucket", bucket)
	log.Info(logger, "Received S3 event")

	ffisData, err := parseFFISData(ctx, bucket, uploadedFile, s3client)
	if err != nil {
		log.Error(logger, "Error parsing FFIS data", err)
		return err
	}

	if err := UpdateOpportunity(ctx, dbapi, env.DestinationTable, opportunity(ffisData)); err != nil {
		return log.Errorf(logger, "Error saving FFIS opportunity data to DynamoDB", err,
			"data", ffisData)
	}

	sendMetric("opportunity.saved", 1)
	return nil
}

func parseFFISData(ctx context.Context, bucket string, uploadedFile string, s3client S3API) (ffis.FFISFundingOpportunity, error) {
	var ffisData ffis.FFISFundingOpportunity

	// get the file from S3
	s3obj, err := s3client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(uploadedFile),
	})
	if err != nil {
		return ffisData, log.Errorf(logger, "Error getting file from S3", err)
	}
	defer s3obj.Body.Close()
	// parse the file contents into JSON
	err = json.NewDecoder(s3obj.Body).Decode(&ffisData)
	if err != nil {
		return ffisData, log.Errorf(logger, "Error decoding file contents", err)
	}

	// validate the data
	if ffisData.Bill == "" {
		return ffisData, ErrMissingBill
	}
	if ffisData.GrantID == 0 {
		return ffisData, ErrMissingGrantID
	}

	log.Info(logger, "Parsed FFIS data", "grant_id", ffisData.GrantID, "bill", ffisData.Bill)
	return ffisData, nil
}
