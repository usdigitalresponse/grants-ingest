package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
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

type MockSQS struct {
	message *string
}

func (mocksqs *MockSQS) GetQueueUrl(ctx context.Context,
	params *sqs.GetQueueUrlInput,
	optFns ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error) {
	output := &sqs.GetQueueUrlOutput{
		QueueUrl: aws.String("https://sqs.us-east-1.amazonaws.com/123456789012/MyQueue"),
	}
	return output, nil
}

func (mocksqs *MockSQS) SendMessage(ctx context.Context,
	params *sqs.SendMessageInput,
	optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	mocksqs.message = params.MessageBody
	output := &sqs.SendMessageOutput{
		MessageId: aws.String("123456789012345678901234567890"),
	}
	return output, nil
}

func TestHandleS3Event(t *testing.T) {
	logger = log.NewNopLogger()
	urlPattern = "https://mcusercontent.com/.+\\.xlsx"
	var tests = []struct {
		emailFixture, expectedURL string
	}{
		{"good.eml", "https://mcusercontent.com/123456/files/file-01.xlsx"},
		{"missing.eml", ""},
		{"multiple.eml", ""},
	}

	for _, test := range tests {
		content, err := os.ReadFile("./fixtures/" + test.emailFixture)
		if err != nil {
			t.Errorf("Error opening file: %v", err)
		}
		mocks3, mocksqs := getMockClients()
		mocks3.content = string(content)
		ctx := context.Background()
		cfg := aws.Config{}
		s3Event := events.S3Event{
			Records: []events.S3EventRecord{
				{
					S3: events.S3Entity{
						Bucket: events.S3Bucket{
							Name: "test-bucket",
						},
						Object: events.S3Object{
							Key: "test/email/file.eml",
						},
					},
				},
			},
		}

		err = handleS3EventWithConfig(cfg, ctx, s3Event, mocks3, mocksqs)

		if test.expectedURL != "" {
			if err != nil {
				t.Errorf("Error parsing S3 event: %v", err)
			}
			if *mocksqs.message != test.expectedURL {
				t.Errorf("Expected message %v, got %v", test.expectedURL, mocksqs.message)
			}
		} else {
			// parse expected bad message
			if mocksqs.message == nil && test.expectedURL != "" {
				t.Errorf("Expected message for %s to be empty", test.emailFixture)
			}
		}
	}
}

func getMockClients() (*MockS3, *MockSQS) {
	mocks3 := MockS3{content: "test"}
	mocksqs := MockSQS{}
	return &mocks3, &mocksqs
}
