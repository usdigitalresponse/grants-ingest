package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-kit/log"
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

func TestParseFFISData(t *testing.T) {
	logger = log.NewNopLogger()
	var tests = []struct {
		jsonFixture, expectedBill, expectedOpportunity string
		expectedError                                  error
	}{
		{"standard.json", "HR 1234", "HR001", nil},
		{"extra-fields.json", "HR 5678", "HR002", nil},
		{"malformed.json", "", "", fmt.Errorf("Error parsing FFIS data")},
		{"missing-fields-bill.json", "", "", ErrMissingBill},
		{"missing-fields-opp.json", "", "", ErrMissingOppNumber},
	}
	for _, test := range tests {
		t.Run(test.jsonFixture, func(t *testing.T) {
			content, err := os.ReadFile("./fixtures/" + test.jsonFixture)
			if err != nil {
				t.Errorf("Error opening file: %v", err)
			}
			mocks3 := getMockClients()
			mocks3.content = string(content)
			results, err := parseFFISData(context.Background(), "test", mocks3)
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
				if ffisData.OppNumber != test.expectedOpportunity {
					t.Errorf("Expected opportunity number %s, got %s", test.expectedOpportunity, ffisData.OppNumber)
				}
			}

		})
	}
}

func getMockClients() *MockS3 {
	mocks3 := MockS3{content: "test"}
	return &mocks3
}
