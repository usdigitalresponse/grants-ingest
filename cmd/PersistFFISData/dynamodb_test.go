package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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
		name, bill, opportunityNumber string
		expectedError                 error
	}{
		{"standard", "HR 1234", "HR001", nil},
		{"error updating", "HR 5678", "HR002", fmt.Errorf("Error updating DynamoDB")},
		//	{"", "", fmt.Errorf("Error parsing FFIS data")},
		//	{"", "", ErrMissingBill},
		//	{"", "", ErrMissingOppNumber},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tableName := "test-table"
			opp := opportunity{
				OppNumber: test.opportunityNumber,
				Bill:      test.bill,
			}
			mock := mockDynamoDBUpdateItemAPI{expectedError: test.expectedError}
			result := UpdateDynamoDBItem(context.TODO(), &mock, tableName, opp)

			if result != test.expectedError {
				t.Errorf("Expected error %v, got %v", test.expectedError, result)
			}

			passedParams := mock.params
			if *passedParams.TableName != tableName {
				t.Errorf("Expected table name %v, got %v", tableName, mock.params.TableName)
			}
			if passedParams.Key["opportunity_number"].(*types.AttributeValueMemberS).Value != test.opportunityNumber {
				t.Errorf("Expected opportunity number %v, got %v", test.opportunityNumber, passedParams.Key["grant_number"].(*types.AttributeValueMemberS).Value)
			}
			println(passedParams.ExpressionAttributeValues["bill"])
			if passedParams.ExpressionAttributeValues["bill"].(*types.AttributeValueMemberS).Value != test.bill {
				t.Errorf("Expected bill %v, got %v", test.bill, passedParams.ExpressionAttributeValues[":bill"].(*types.AttributeValueMemberS).Value)
			}

		})
	}

}
