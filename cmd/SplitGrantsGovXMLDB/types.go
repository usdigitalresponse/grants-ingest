package main

import (
	"encoding/xml"
	"fmt"
	"time"

	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	grantsgov "github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/grants.gov"
)

type grantRecord interface {
	logWith(log.Logger) log.Logger
	// s3ObjectKey returns a string to use as the object key when saving the opportunity to an S3 bucket
	s3ObjectKey() string
	dynamoDBItemKey() map[string]ddbtypes.AttributeValue
	lastModified() (time.Time, error)
	toXML() ([]byte, error)
}

type opportunity grantsgov.OpportunitySynopsisDetail_1_0

func (o opportunity) logWith(logger log.Logger) log.Logger {
	return log.With(logger,
		"opportunity_id", o.OpportunityID,
		"opportunity_number", o.OpportunityNumber,
		"is_forecast", false,
	)
}

func (o opportunity) s3ObjectKey() string {
	return fmt.Sprintf("%s/%s/grants.gov/v2.OpportunitySynopsisDetail_1_0.xml",
		o.OpportunityID[0:3], o.OpportunityID,
	)
}

func (o opportunity) dynamoDBItemKey() map[string]ddbtypes.AttributeValue {
	return map[string]ddbtypes.AttributeValue{
		"grant_id": &ddbtypes.AttributeValueMemberS{Value: string(o.OpportunityID)},
	}
}

func (o opportunity) lastModified() (time.Time, error) {
	return o.LastUpdatedDate.Time()
}

func (o opportunity) toXML() ([]byte, error) {
	return xml.Marshal(grantsgov.OpportunitySynopsisDetail_1_0(o))
}

type forecast grantsgov.OpportunityForecastDetail_1_0

func (f forecast) logWith(logger log.Logger) log.Logger {
	return log.With(logger,
		"opportunity_id", f.OpportunityID,
		"opportunity_number", f.OpportunityNumber,
		"is_forecast", true,
	)
}

func (f forecast) s3ObjectKey() string {
	return fmt.Sprintf("%s/%s/grants.gov/v2.OpportunityForecastDetail_1_0.xml",
		f.OpportunityID[0:3], f.OpportunityID,
	)
}

func (f forecast) lastModified() (time.Time, error) {
	return f.LastUpdatedDate.Time()
}

func (f forecast) toXML() ([]byte, error) {
	return xml.Marshal(grantsgov.OpportunityForecastDetail_1_0(f))
}

func (f forecast) dynamoDBItemKey() map[string]ddbtypes.AttributeValue {
	return map[string]ddbtypes.AttributeValue{
		"grant_id": &ddbtypes.AttributeValueMemberS{Value: string(f.OpportunityID)},
	}
}
