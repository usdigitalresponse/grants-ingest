package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/usdigitalresponse/grants-ingest/internal/awsHelpers"
)

type DynamoDBUpdateItemAPI interface {
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

func UpdateDynamoDBItem(ctx context.Context, c DynamoDBUpdateItemAPI, table string, opp opportunity) error {
	key, err := buildKey(opp)
	if err != nil {
		return err
	}
	expr, err := buildUpdateExpression(opp)
	if err != nil {
		return err
	}
	_, err = c.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(table),
		Key:                       key,
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
		ReturnValues:              types.ReturnValueUpdatedNew,
	})
	return err
}

func buildKey(o opportunity) (map[string]types.AttributeValue, error) {
	oid, err := attributevalue.Marshal(o.OpportunityID)

	return map[string]types.AttributeValue{"grant_id": oid}, err
}

func buildUpdateExpression(o opportunity) (expression.Expression, error) {
	oppAttr, err := attributevalue.MarshalMap(o)
	if err != nil {
		return expression.Expression{}, err
	}

	update := expression.UpdateBuilder{}
	for k, v := range oppAttr {
		update = update.Set(expression.Name(k), expression.Value(v))
	}
	update = awsHelpers.DDBSetRevisionForUpdate(update)
	condition, err := awsHelpers.DDBIfAnyValueChangedCondition(oppAttr)
	if err != nil {
		return expression.Expression{}, err
	}

	return expression.NewBuilder().WithUpdate(update).WithCondition(condition).Build()
}
