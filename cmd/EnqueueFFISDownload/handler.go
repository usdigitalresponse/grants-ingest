package main

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/ffis"
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

// error constants
var (
	ErrNoMatchesFound = fmt.Errorf("no matches found")
	ErrMultipleFound  = fmt.Errorf("multiple matches found")
	ErrNoPlaintext    = fmt.Errorf("no plaintext mime part found")
)

func handleS3Event(ctx context.Context, s3Event events.S3Event, s3client S3API, sqsclient SQSAPI) error {
	uploadedFile := s3Event.Records[0].S3.Object.Key
	emailBody, err := getEmailFromS3Event(ctx, s3client, s3Event, uploadedFile)
	if err != nil {
		return log.Errorf(logger, "Error reading email from S3", err)
	}
	defer emailBody.Close()
	plaintext, err := plaintextMIMEFromEmailBody(emailBody)
	if err != nil {
		return log.Errorf(logger, "Missing plaintext mime part from email body", err)
	}
	// Parse the URL from the email body
	url, err := parseURLFromEmailBody(plaintext)
	if err != nil {
		return log.Errorf(logger, "Download URL could not be located in email plaintext", err)
	}

	log.Info(logger, "Parsed URL from email body", "url", url)

	// Enqueue the URL for download
	err = enqueueURLForDownload(ctx, sqsclient, url, uploadedFile)
	if err != nil {
		return log.Errorf(logger, "Failed to enqueue parsed URL", err)
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

	return "", ErrNoPlaintext
}

func parseURLFromEmailBody(plaintext string) (string, error) {
	patternRegex := regexp.MustCompile(env.URLPattern)
	matches := patternRegex.FindAllString(plaintext, -1)
	if len(matches) == 0 {
		return "", ErrNoMatchesFound
	} else if len(matches) > 1 {
		return "", ErrMultipleFound
	}
	return matches[0], nil
}

func enqueueURLForDownload(ctx context.Context, client SQSAPI, url string, fileKey string) error {
	messageObj := ffis.FFISMessageDownload{
		DownloadURL:   url,
		SourceFileKey: fileKey,
	}
	serializedMessage, err := json.Marshal(messageObj)
	if err != nil {
		return err
	}

	message := sqs.SendMessageInput{
		MessageBody: aws.String(string(serializedMessage)),
		QueueUrl:    aws.String(env.DestinationQueueURL),
	}

	output, err := client.SendMessage(ctx, &message)

	if err != nil {
		return err
	}
	log.Info(logger, "Sent SQS message", "messageId", *output.MessageId)
	return nil
}

func getEmailFromS3Event(ctx context.Context, s3client S3API, s3Event events.S3Event, uploadedFileName string) (io.ReadCloser, error) {
	bucket := s3Event.Records[0].S3.Bucket.Name

	logger := log.With(logger, "bucket", bucket, "key", uploadedFileName)
	log.Debug(logger, "Reading from bucket")
	// Get the email body
	resp, err := s3client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(uploadedFileName),
	})
	if err != nil {
		return nil, err
	}
	log.Info(logger, "Retrieved new email file")
	return resp.Body, nil
}
