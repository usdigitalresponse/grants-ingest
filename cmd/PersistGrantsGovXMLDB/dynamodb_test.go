package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/usdigitalresponse/grants-ingest/internal/awsHelpers"
)

type mockUpdateItemAPI func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)

func (m mockUpdateItemAPI) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	return m(ctx, params, optFns...)
}

type mockDynamoDBUpdateItemAPI struct {
	mockUpdateItemAPI
}

func TestUploadDynamoDBItem(t *testing.T) {
	testTableName := "test-table"
	testError := fmt.Errorf("oh no this is an error")
	testItemAttrs := map[string]types.AttributeValue{
		"someKey":    &types.AttributeValueMemberS{Value: "is a key"},
		"attrString": &types.AttributeValueMemberS{Value: "is a string"},
		"attrBool":   &types.AttributeValueMemberBOOL{Value: true},
	}
	testKey := map[string]types.AttributeValue{"someKey": testItemAttrs["someKey"]}

	for _, tt := range []struct {
		name       string
		key, attrs map[string]types.AttributeValue
		client     func(t *testing.T) DynamoDBUpdateItemAPI
		expErr     error
	}{
		{
			"UpdateItem successful",
			testKey,
			testItemAttrs,
			func(t *testing.T) DynamoDBUpdateItemAPI {
				return mockUpdateItemAPI(func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
					t.Helper()
					assert.Equal(t, testKey, params.Key)
					return &dynamodb.UpdateItemOutput{}, nil
				})
			},
			nil,
		},
		{
			"UpdateItem returns error",
			testKey,
			testItemAttrs,
			func(t *testing.T) DynamoDBUpdateItemAPI {
				return mockUpdateItemAPI(func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
					t.Helper()
					assert.Equal(t, aws.String(testTableName), params.TableName)
					assert.Equal(t, testKey, params.Key)
					return &dynamodb.UpdateItemOutput{}, testError
				})
			},
			testError,
		},
		{
			"Empty attribute map returns error",
			testKey,
			make(map[string]types.AttributeValue),
			func(t *testing.T) DynamoDBUpdateItemAPI {
				return mockUpdateItemAPI(func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
					t.Helper()
					assert.Equal(t, aws.String(testTableName), params.TableName)
					assert.Equal(t, testKey, params.Key)
					require.Fail(t, "UpdateItem called unexpectedly")
					return &dynamodb.UpdateItemOutput{}, nil
				})
			},
			awsHelpers.ErrEmptyFields,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := UpdateDynamoDBItem(context.TODO(), tt.client(t), testTableName, tt.key, tt.attrs)
			if tt.expErr != nil {
				assert.EqualError(t, err, tt.expErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
