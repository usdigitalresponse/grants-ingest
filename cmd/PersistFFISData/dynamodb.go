package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/ffis"
)

type DynamoDBUpdateItemAPI interface {
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

type opportunity ffis.FFISFundingOpportunity

func UpdateOpportunity(ctx context.Context, c DynamoDBUpdateItemAPI, table string, opp opportunity) error {
	key, err := buildKey(opp)
	if err != nil {
		return err
	}

	update := expression.Set(expression.Name("bill"), expression.Value(opp.Bill))

	expr, err := expression.NewBuilder().WithUpdate(update).Build()
	if err != nil {
		return err
	}

	_, err = c.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(table),
		Key:                       key,
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
		ReturnValues:              types.ReturnValueNone,
	})
	return err
}

func buildKey(o opportunity) (map[string]types.AttributeValue, error) {
	grant_id, err := attributevalue.Marshal(o.GrantID)

	return map[string]types.AttributeValue{"grant_id": grant_id}, err
}
