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
	SendMessage(ctx context.Context,
		params *sqs.SendMessageInput,
		optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

type S3API interface {
	GetObject(ctx context.Context,
		params *s3.GetObjectInput,
		optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

func handleS3Event(ctx context.Context, s3Event events.S3Event, s3client S3API, sqsclient SQSAPI) error {
	emailBody, err := getEmailFromS3Event(s3client, s3Event, ctx)
	if err != nil {
		return err
	}
	defer emailBody.Close()
	plaintext, err := plaintextMIMEFromEmailBody(emailBody)
	if err != nil {
		return err
	}
	// Parse the URL from the email body
	url, err := parseURLFromEmailBody(plaintext)
	if err != nil {
		return err
	}

	log.Info(logger, "Parsed URL from email body", "url", url)

	// Enqueue the URL for download
	err = enqueueURLForDownload(url, sqsclient, ctx)
	if err != nil {
		return err
	}

	return nil
}

func plaintextMIMEFromEmailBody(email io.Reader) (string, error) {
	msg, err := mail.ReadMessage(email)
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

func parseURLFromEmailBody(plaintext string) (string, error) {
	patternRegex := regexp.MustCompile(env.URLPattern)
	matches := patternRegex.FindStringSubmatch(plaintext)
	if len(matches) == 0 {
		return "", fmt.Errorf("no matches found")
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple matches found")
	}
	return matches[0], nil
}

func enqueueURLForDownload(url string, client SQSAPI, ctx context.Context) error {
	message := sqs.SendMessageInput{
		MessageBody: aws.String(url),
		QueueUrl:    aws.String(env.DestinationQueueURL),
	}
	output, err := client.SendMessage(ctx, &message)

	if err != nil {
		return err
	}
	log.Info(logger, "Sent SQS message", "messageId", *output.MessageId)
	return nil
}

func getEmailFromS3Event(s3client S3API, s3Event events.S3Event, ctx context.Context) (io.ReadCloser, error) {
	bucket := s3Event.Records[0].S3.Bucket.Name
	log.Debug(logger, "Reading from bucket", "bucket", bucket)
	uploadedFile := s3Event.Records[0].S3.Object.Key
	log.Info(logger, "New email file", "file", uploadedFile)
	// Get the email body
	resp, err := s3client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(uploadedFile),
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
