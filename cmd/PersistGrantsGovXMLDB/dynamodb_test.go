package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	grantsgov "github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/grants.gov"
)

type mockUpdateItemAPI func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)

func (m mockUpdateItemAPI) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	return m(ctx, params, optFns...)
}

type mockDynamoDBUpdateItemAPI struct {
	mockUpdateItemAPI
}

func TestUploadDynamoDBItem(t *testing.T) {
	now := time.Now()
	testTableName := "test-table"
	testHashKey := map[string]types.AttributeValue{}
	testHashKey["grant_id"] = &types.AttributeValueMemberS{Value: "123456"}
	testError := fmt.Errorf("oh no this is an error")
	testOpportunity := opportunity{
		OpportunityID:   "123456",
		LastUpdatedDate: grantsgov.MMDDYYYYType(now.Format(grantsgov.TimeLayoutMMDDYYYYType)),
	}

	for _, tt := range []struct {
		name   string
		client func(t *testing.T) DynamoDBUpdateItemAPI
		expErr error
	}{
		{
			"UpdateItem successful",
			func(t *testing.T) DynamoDBUpdateItemAPI {
				return mockUpdateItemAPI(func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
					t.Helper()
					assert.Equal(t, aws.String(testTableName), params.TableName)
					assert.Equal(t, testHashKey, params.Key)
					return &dynamodb.UpdateItemOutput{}, nil
				})
			},
			nil,
		},
		{
			"UpdateItem returns error",
			func(t *testing.T) DynamoDBUpdateItemAPI {
				return mockUpdateItemAPI(func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
					t.Helper()
					assert.Equal(t, aws.String(testTableName), params.TableName)
					assert.Equal(t, testHashKey, params.Key)
					return &dynamodb.UpdateItemOutput{}, testError
				})
			},
			testError,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := UpdateDynamoDBItem(context.TODO(), tt.client(t), testTableName, testOpportunity)
			if tt.expErr != nil {
				assert.EqualError(t, err, tt.expErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
