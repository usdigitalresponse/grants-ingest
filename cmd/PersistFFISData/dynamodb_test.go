package main

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type mockDynamoDBUpdateItemAPI struct {
	expectedError error
	params        *dynamodb.UpdateItemInput
}

func (m *mockDynamoDBUpdateItemAPI) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	m.params = params
	return &dynamodb.UpdateItemOutput{}, m.expectedError
}

func TestUpsertDynamoDB(t *testing.T) {
	var tests = []struct {
		name, bill    string
		grantId       int64
		expectedError error
	}{
		{"standard", "HR 1234", 123, nil},
		{"error updating", "HR 5678", 456, fmt.Errorf("Error updating DynamoDB")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tableName := "test-table"
			opp := opportunity{
				GrantID: test.grantId,
				Bill:    test.bill,
			}
			mock := mockDynamoDBUpdateItemAPI{expectedError: test.expectedError}
			result := UpdateOpportunity(context.TODO(), &mock, tableName, opp)

			if result != test.expectedError {
				t.Errorf("Expected error %v, got %v", test.expectedError, result)
			}

			passedParams := mock.params
			if *passedParams.TableName != tableName {
				t.Errorf("Expected table name %v, got %v", tableName, *passedParams.TableName)
			}
			key := make(map[string]string)
			attributevalue.UnmarshalMap(passedParams.Key, &key)
			if key["grant_id"] != strconv.FormatInt(test.grantId, 10) {
				t.Errorf("Expected grant_id %v, got %v", test.grantId, key["grant_id"])
			}
			values := make(map[string]string)
			attributevalue.UnmarshalMap(passedParams.ExpressionAttributeValues, &values)
			if !checkMapContainsValue(t, values, test.bill) {
				t.Error("Missing bill value in update attribute values")
			}
			if !checkMapContainsValue(t, passedParams.ExpressionAttributeNames, "revision") {
				t.Errorf("Missing attribute %q in update attribute names", "revision")
			}
		})
	}
}

// checkMapContainsValue is a testing helper function that returns true if target is a value of m
func checkMapContainsValue[K comparable, V comparable](t *testing.T, m map[K]V, target V) bool {
	t.Helper()
	for _, v := range m {
		if v == target {
			return true
		}
	}
	return false
}
