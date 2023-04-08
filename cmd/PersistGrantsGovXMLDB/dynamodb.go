package main

// import (
// 	"context"
// 	"io"

// 	"github.com/aws/aws-sdk-go-v2/aws"
// 	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
// )

// type DynamoDBUpdateItemAPI interface {
// 	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
// }

// func UpdateDynamoDBItem(ctx context.Context, c DynamoDBUpdateItemAPI, table, key string, r io.Reader) error {
// 	_, err := c.UpdateItem(ctx, &dynamodb.UpdateItemInput{
// 		ExpressionAttributeValues: expr,
// 		TableName:                 aws.String(table),
// 		Key:                       key,
// 		ReturnValues:              aws.String("UPDATED_NEW"),
// 		UpdateExpression:          aws.String("set info.rating = :r"),
// 	})
// 	return err
// }
