package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grantsgov "github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/grants.gov"
)

type mockDynamoDBGetItemClient func(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)

func (m mockDynamoDBGetItemClient) GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return m(ctx, params, optFns...)
}

func makeTestItem(t *testing.T, lastUpdatedDateTestValue any) map[string]ddbtypes.AttributeValue {
	t.Helper()
	rv, err := attributevalue.MarshalMap(map[string]any{"LastUpdatedDate": lastUpdatedDateTestValue})
	require.NoError(t, err, "Unexpected error creating test DynamoDB item fixture during setup")
	return rv
}

func TestGetDynamoDBLastModified(t *testing.T) {
	testTableName := "test-table"
	testItemKey := map[string]ddbtypes.AttributeValue{
		"grant_id": &ddbtypes.AttributeValueMemberS{Value: "test-key"},
	}
	testLastUpdateDateString := time.Now().Format(grantsgov.TimeLayoutMMDDYYYYType)
	testLastUpdateDate, err := time.Parse(grantsgov.TimeLayoutMMDDYYYYType, testLastUpdateDateString)
	require.NoError(t, err, "Unexpected error parsing time fixture during test setup")
	testInvalidDateString := "not a valid date string"

	_, testInvalidDateStringParseError := time.Parse(grantsgov.TimeLayoutMMDDYYYYType, testInvalidDateString)
	require.Error(t, testInvalidDateStringParseError, "Error fixture unexpectedly nil during test setup")

	for _, tt := range []struct {
		name            string
		ddbItem         map[string]ddbtypes.AttributeValue
		ddbErr          error
		expLastModified *time.Time
		expErr          error
	}{
		{
			"GetItem produces item with valid LastUpdatedDate",
			makeTestItem(t, testLastUpdateDateString),
			nil,
			&testLastUpdateDate,
			nil,
		},
		{
			"GetItem returns error",
			nil,
			errors.New("GetItem action failed"),
			nil,
			errors.New("GetItem action failed"),
		},
		{"GetItem key not found", nil, nil, nil, nil},
		{
			"GetItem produces item with invalid LastUpdateDate",
			makeTestItem(t, testInvalidDateString),
			nil,
			nil,
			testInvalidDateStringParseError,
		},
		{
			"GetItem produces item that cannot be unmarshalled",
			makeTestItem(t, true),
			nil,
			nil,
			errors.New("unmarshal failed, cannot unmarshal bool into Go value type grantsgov.MMDDYYYYType"),
		},
	} {

		t.Run(tt.name, func(t *testing.T) {
			mockClient := mockDynamoDBGetItemClient(func(ctx context.Context, params *dynamodb.GetItemInput, f ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
				require.Equal(t, &testTableName, params.TableName, "Unexpected table name in GetItem params")
				require.Equal(t, testItemKey, params.Key, "Unexpected item key in GetItem params")

				return &dynamodb.GetItemOutput{Item: tt.ddbItem}, tt.ddbErr
			})

			lastModified, err := GetDynamoDBLastModified(context.TODO(),
				mockClient, testTableName, testItemKey)

			if tt.expErr != nil {
				assert.EqualError(t, err, tt.expErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expLastModified, lastModified)
			}
		})
	}
}
