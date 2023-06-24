package main

import (
	"context"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/usdigitalresponse/grants-ingest/internal/awsHelpers"
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
	oppAttr, err := attributevalue.MarshalMap(map[string]interface{}{"Bill": opp.Bill})
	if err != nil {
		return err
	}
	condition, _ := awsHelpers.DDBIfAnyValueChangedCondition(oppAttr)

	update := expression.Set(expression.Name("Bill"), expression.Value(oppAttr["Bill"]))

	expr, err := expression.NewBuilder().WithUpdate(update).WithCondition(condition).Build()
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
		ReturnValues:              types.ReturnValueNone,
	})
	return err
}

func buildKey(o opportunity) (map[string]types.AttributeValue, error) {
	grantIDStr := strconv.FormatInt(o.GrantID, 10)
	grantIDKey, err := attributevalue.Marshal(grantIDStr)

	return map[string]types.AttributeValue{"grant_id": grantIDKey}, err
}
