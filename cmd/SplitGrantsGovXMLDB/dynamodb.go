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

type DynamoDBItemKey map[string]ddbtypes.AttributeValue

type DynamoDBGetItemAPI interface {
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
}

func GetDynamoDBLastModified(ctx context.Context, c DynamoDBGetItemAPI, table string, key DynamoDBItemKey) (*time.Time, error) {
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
