package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockS3 struct {
	content string
}

func (mocks3 *MockS3) GetObject(ctx context.Context,
	params *s3.GetObjectInput,
	optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	contentBytes := []byte(mocks3.content)
	return &s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader(contentBytes)),
		ContentLength: int64(len(contentBytes)),
	}, nil
}

func TestInvocationErrorsUpdatingOpportunity(t *testing.T) {
	logger = log.NewNopLogger()
	content, err := os.ReadFile("./fixtures/standard.json")
	require.NoError(t, err)
	mockS3 := getMockClients()
	mockS3.content = string(content)
	s3Event := events.S3Event{Records: []events.S3EventRecord{{S3: events.S3Entity{
		Bucket: events.S3Bucket{Name: "source-bucket"},
		Object: events.S3Object{Key: "does/not/matter"},
	}}}}

	basicErr := fmt.Errorf("hi, I'm a basic error")
	duplicateItemErr := &types.DuplicateItemException{
		Message: aws.String("Item already exists"),
	}
	conditionalCheckErr := &types.ConditionalCheckFailedException{
		Message: aws.String("The conditional request failed"),
	}
	for _, tt := range []struct {
		name                  string
		ddbErr, invocationErr error
	}{
		{"fails basic error", basicErr, basicErr},
		{"fails on duplicate item error", duplicateItemErr, duplicateItemErr},
		{"ignores conditional check error", conditionalCheckErr, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err = handleS3Event(context.Background(), s3Event, mockS3, &mockDynamoDBUpdateItemAPI{
				expectedError: tt.ddbErr,
			})
			if tt.invocationErr == nil {
				require.NoError(t, err)
			}
			assert.ErrorIs(t, err, tt.invocationErr)
		})
	}

}

func TestParseFFISData(t *testing.T) {
	logger = log.NewNopLogger()
	var tests = []struct {
		jsonFixture, expectedBill string
		expectedGrantId           int64
		expectedError             error
	}{
		{"standard.json", "HR 1234", 123, nil},
		{"extra-fields.json", "HR 5678", 1234, nil},
		{"malformed.json", "", 0, fmt.Errorf("Error decoding file contents")},
		{"missing-fields-bill.json", "", 0, ErrMissingBill},
		{"missing-fields-grant.json", "", 0, ErrMissingGrantID},
	}
	for _, test := range tests {
		t.Run(test.jsonFixture, func(t *testing.T) {
			content, err := os.ReadFile("./fixtures/" + test.jsonFixture)
			if err != nil {
				t.Errorf("Error opening file: %v", err)
			}
			mocks3 := getMockClients()
			mocks3.content = string(content)
			results, err := parseFFISData(context.Background(), "test", "bucket", mocks3)
			if err != nil {
				if test.expectedError == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if !strings.Contains(err.Error(), test.expectedError.Error()) {
					t.Errorf("Expected error %v, got %v", test.expectedError, err)
				}
			} else {
				if test.expectedError != nil {
					t.Errorf("Expected error %v, got nil", test.expectedError)
				}
				ffisData := results
				if ffisData.Bill != test.expectedBill {
					t.Errorf("Expected bill %s, got %s", test.expectedBill, ffisData.Bill)
				}
				if ffisData.GrantID != test.expectedGrantId {
					t.Errorf("Expected grant id %v, got %v", test.expectedGrantId, ffisData.GrantID)
				}
			}
		})
	}
}

func getMockClients() *MockS3 {
	mocks3 := MockS3{content: "test"}
	return &mocks3
}
