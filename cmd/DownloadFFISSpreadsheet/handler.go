package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type SQSAPI interface {
	SendMessage(ctx context.Context,
		params *sqs.SendMessageInput,
		optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

type S3API interface {
	GetObject(ctx context.Context,
		params *s3.GetObjectInput,
		optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

func handleSQSEvent(ctx context.Context, sqsEvent events.SQSEvent, s3client S3API, sqsclient SQSAPI) error {
	msg := sqsEvent.Records[0].Body
	fmt.Println(msg)
	return fmt.Errorf("not implemented")
}
