package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoDBUpdateItemAPI interface {
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

func UpdateDynamoDBItem(ctx context.Context, c DynamoDBUpdateItemAPI, table string, opp opportunity) error {
	key, err := opp.buildKey()
	if err != nil {
		return err
	}
	expr, err := opp.buildUpdateExpression()
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

func (o opportunity) buildKey() (map[string]types.AttributeValue, error) {
	oid, err := attributevalue.Marshal(o.OpportunityID)
	if err != nil {
		panic(err)
	}

	return map[string]types.AttributeValue{"grant_id": oid}, err
}

func (o opportunity) buildUpdateExpression() (expression.Expression, error) {
	oppAttr, err := attributevalue.MarshalMap(o)
	if err != nil {
		panic(err)
	}

	update := expression.UpdateBuilder{}
	for k, v := range oppAttr {
		update = update.Set(expression.Name(k), expression.Value(v))
	}

	return expression.NewBuilder().WithUpdate(update).Build()
}
