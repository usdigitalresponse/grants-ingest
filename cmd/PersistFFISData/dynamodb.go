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

func UpdateDynamoDBItem(ctx context.Context, c DynamoDBUpdateItemAPI, table string, opp opportunity) error {
	key, err := buildKey(opp)
	if err != nil {
		return err
	}
	expr, err := buildUpdateExpression(opp, map[string]string{
		"Bill": "bill",
	})
	if err != nil {
		return err
	}
	// println(*expr.Update())
	// println(expr.Names())
	// println(expr.Values())
	for k, v := range expr.Values() {
		println(k, v)
	}
	println(*expr.Update())
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
	oid, err := attributevalue.Marshal(o.OppNumber)

	return map[string]types.AttributeValue{"opportunity_number": oid}, err
}

func buildUpdateExpression(o opportunity, attrs map[string]string) (expression.Expression, error) {
	oppAttr, err := attributevalue.MarshalMap(o)
	if err != nil {
		return expression.Expression{}, err
	}

	update := expression.UpdateBuilder{}
	for oppKey, tableKey := range attrs {
		update = update.Set(expression.Name(tableKey), expression.Value(oppAttr[oppKey]))
	}
	return expression.NewBuilder().WithUpdate(update).Build()
}
