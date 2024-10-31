package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/usdigitalresponse/grants-ingest/internal/awsHelpers"
)

type DynamoDBUpdateItemAPI interface {
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

func UpdateDynamoDBItem(ctx context.Context, c DynamoDBUpdateItemAPI, table string, key, attrs map[string]types.AttributeValue) error {
	expr, err := buildUpdateExpression(attrs)
	if err != nil {
		return err
	}
	_, err = c.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(table),
		Key:                       key,
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
		ReturnValues:              types.ReturnValueUpdatedNew,
	})
	return err
}

func buildUpdateExpression(m map[string]types.AttributeValue) (expression.Expression, error) {
	update := expression.UpdateBuilder{}
	for k, v := range m {
		update = update.Set(expression.Name(k), expression.Value(v))
	}
	update = awsHelpers.DDBSetRevisionForUpdate(update)
	condition, err := awsHelpers.DDBIfAnyValueChangedCondition(m)
	if err != nil {
		return expression.Expression{}, err
	}

	return expression.NewBuilder().WithUpdate(update).WithCondition(condition).Build()
}
