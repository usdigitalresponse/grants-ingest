package main

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	grantsgov "github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/grants.gov"
)

// DynamoDBGetItemAPI is the interface for retrieving a single item from a DynamoDB table via primary key lookup
type DynamoDBGetItemAPI interface {
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
}

// GetDynamoDBLastModified gets the "Last Modified" timestamp for a grantRecord stored in a DynamoDB table.
// If the item exists, a pointer to the last modification time is returned along with a nil error.
// If the specified item does not exist, the returned *time.Time and error are both nil.
// If an error is encountered when calling the HeadObject S3 API method, this will return a nil
// *time.Time value along with the encountered error.
func GetDynamoDBLastModified(ctx context.Context, c DynamoDBGetItemAPI, table string, key map[string]ddbtypes.AttributeValue) (*time.Time, error) {
	resp, err := c.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:            &table,
		Key:                  key,
		ProjectionExpression: aws.String("LastUpdatedDate"),
	})
	if err != nil {
		return nil, err
	}
	if resp.Item == nil {
		return nil, nil
	}

	item := struct{ LastUpdatedDate grantsgov.MMDDYYYYType }{}
	if err := attributevalue.UnmarshalMap(resp.Item, &item); err != nil {
		return nil, err
	}
	lastUpdatedDate, err := item.LastUpdatedDate.Time()
	return &lastUpdatedDate, err
}
