package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"regexp"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
)

type SQSAPI interface {
	GetQueueUrl(ctx context.Context,
		params *sqs.GetQueueUrlInput,
		optFns ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error)

	SendMessage(ctx context.Context,
		params *sqs.SendMessageInput,
		optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

type S3API interface {
	GetObject(ctx context.Context,
		params *s3.GetObjectInput,
		optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

func handleS3EventWithConfig(cfg aws.Config, ctx context.Context, s3Event events.S3Event, s3client S3API, sqsclient SQSAPI) error {
	emailBody, err := getEmailFromS3Event(s3client, s3Event, ctx)
	if err != nil {
		return err
	}

	// Parse the URL from the email body
	url, err := parseURLFromEmailBody(emailBody)
	if err != nil {
		return err
	}

	log.Info(logger, url, "Parsed URL from email body")

	// Enqueue the URL for download
	err = enqueueURLForDownload(url, sqsclient, ctx)
	if err != nil {
		return err
	}

	return nil
}

func plaintextMIMEFromEmailBody(email string) (string, error) {
	reader := strings.NewReader(email)
	msg, err := mail.ReadMessage(reader)
	if err != nil {
		return "", err
	}
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if mediaType != "multipart/alternative" {
		return "", fmt.Errorf("expected multipart/alternative, got %s", mediaType)
	}
	if err != nil {
		return "", err
	}
	mr := multipart.NewReader(msg.Body, params["boundary"])
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(p.Header.Get("Content-Type"), "text/plain") {
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(p)
			if err != nil {
				return "", err
			}
			return buf.String(), nil
		}
	}

	return "", fmt.Errorf("no text/plain part found")
}

func parseURLFromEmailBody(email string) (string, error) {
	plaintext, err := plaintextMIMEFromEmailBody(email)
	if err != nil {
		return "", err
	}
	patternRegex := regexp.MustCompile(urlPattern)
	matches := patternRegex.FindStringSubmatch(plaintext)
	if len(matches) == 0 {
		return "", fmt.Errorf("no matches found")
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple matches found")
	}
	return matches[0], nil
}

func getQueueUrl(queueName string, client SQSAPI, ctx context.Context) (*string, error) {
	result, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	},
	)
	if err != nil {
		return nil, err
	}
	return result.QueueUrl, nil

}

func enqueueURLForDownload(url string, client SQSAPI, ctx context.Context) error {
	queueUrl, err := getQueueUrl(destinationQueue, client, ctx)
	if err != nil {
		return err
	}
	log.Info(logger, *queueUrl, "Got SQS queue URL")

	message := sqs.SendMessageInput{
		MessageBody: aws.String(url),
		QueueUrl:    aws.String(*queueUrl),
	}
	output, err := client.SendMessage(ctx, &message)

	if err != nil {
		return err
	}
	log.Info(logger, *output.MessageId, "Sent SQS message")
	return nil
}

func getEmailFromS3Event(s3client S3API, s3Event events.S3Event, ctx context.Context) (string, error) {
	bucket := s3Event.Records[0].S3.Bucket.Name
	log.Debug(logger, bucket, "Reading from bucket")
	uploadedFile := s3Event.Records[0].S3.Object.Key
	log.Info(logger, uploadedFile, "New email file")
	// Get the email body
	resp, err := s3client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(uploadedFile),
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return "", err
	}
	emailBody := buf.String()
	return emailBody, nil
}
