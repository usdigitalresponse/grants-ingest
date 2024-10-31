package main

import (
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	grantsgov "github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/grants.gov"
)

const (
	GRANT_OPPORTUNITY_XML_NAME = "OpportunitySynopsisDetail_1_0"
	GRANT_FORECAST_XML_NAME    = "OpportunityForecastDetail_1_0"
)

type grantRecord interface {
	logWith(log.Logger) log.Logger
	dynamoDBItemKey() map[string]ddbtypes.AttributeValue
	// Marshalls the grantRecord contents to a map of DynamoDB item attributes,
	// which should contain an additional `is_forcast` discriminator field that can be used
	// to differentiate between `opportunity` and `forecast` source record types.
	dynamoDBAttributeMap() (map[string]ddbtypes.AttributeValue, error)
}

type opportunity grantsgov.OpportunitySynopsisDetail_1_0

func (o opportunity) logWith(logger log.Logger) log.Logger {
	return log.With(logger,
		"opportunity_id", o.OpportunityID,
		"opportunity_number", o.OpportunityNumber,
		"is_forecast", false,
	)
}

func (o opportunity) dynamoDBItemKey() map[string]ddbtypes.AttributeValue {
	return map[string]ddbtypes.AttributeValue{
		"grant_id": &ddbtypes.AttributeValueMemberS{Value: string(o.OpportunityID)},
	}
}

func (o opportunity) dynamoDBAttributeMap() (map[string]ddbtypes.AttributeValue, error) {
	m, err := attributevalue.MarshalMap(o)
	if m != nil {
		m["is_forecast"] = &ddbtypes.AttributeValueMemberBOOL{Value: false}
	}
	return m, err
}

type forecast grantsgov.OpportunityForecastDetail_1_0

func (f forecast) logWith(logger log.Logger) log.Logger {
	return log.With(logger,
		"opportunity_id", f.OpportunityID,
		"opportunity_number", f.OpportunityNumber,
		"is_forecast", true,
	)
}

func (f forecast) dynamoDBItemKey() map[string]ddbtypes.AttributeValue {
	return map[string]ddbtypes.AttributeValue{
		"grant_id": &ddbtypes.AttributeValueMemberS{Value: string(f.OpportunityID)},
	}
}

func (f forecast) dynamoDBAttributeMap() (map[string]ddbtypes.AttributeValue, error) {
	m, err := attributevalue.MarshalMap(f)
	if m != nil {
		m["is_forecast"] = &ddbtypes.AttributeValueMemberBOOL{Value: true}
	}
	return m, err
}
