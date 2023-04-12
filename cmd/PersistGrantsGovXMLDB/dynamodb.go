package main

import (
	"context"
	"reflect"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
)

type DynamoDBUpdateItemAPI interface {
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

func UpdateDynamoDBItem(ctx context.Context, c DynamoDBUpdateItemAPI, table string, opp opportunity) error {
	// expr, err := opp.buildUpdateExpression()
	// if err != nil {
	// 	return err
	// }
	_, err := c.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(table),
		Key:       opp.GetKey(),
		// ExpressionAttributeNames:  expr.Names(),
		// ExpressionAttributeValues: expr.Values(),
		// UpdateExpression:          expr.Update(),
		ReturnValues: types.ReturnValueUpdatedNew,
	})
	return err
}

func (o opportunity) GetKey() map[string]types.AttributeValue {
	oid, err := attributevalue.Marshal(o.OpportunityID)
	if err != nil {
		panic(err)
	}
	return map[string]types.AttributeValue{"grant_id": oid}
}

func (o opportunity) buildUpdateExpression() (expression.Expression, error) {
	v := reflect.ValueOf(o)

	log.Debug(logger, "LOOK AT ME", v)
	update := expression.UpdateBuilder{}
	for i := 0; i < v.NumField(); i++ {
		name := v.Type().Field(i).Name
		value := v.Field(i).Interface()
		update.Set(expression.Name(name), expression.IfNotExists(expression.Name(name), expression.Value(value)))
	}
	return expression.NewBuilder().WithUpdate(update).Build()
}
