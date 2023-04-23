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
	_ "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
)

func plaintextMIMEFromEmailBody(email string) (string, error) {
	println("plaintextMIMEFromEmailBody")
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
		println("looping")
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		println("Content-Type: " + p.Header.Get("Content-Type"))
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
	patternRegex := regexp.MustCompile(`https://mcusercontent.com/.+\.xlsx`)
	matches := patternRegex.FindStringSubmatch(plaintext)
	if len(matches) == 0 {
		return "", fmt.Errorf("no matches found")
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple matches found")
	}
	return matches[0], nil
}

func enqueueURLForDownload(url string, client *sqs.Client, ctx context.Context) error {
	message := sqs.SendMessageInput{
		MessageBody: aws.String(url),
		QueueUrl:    aws.String(env.DestinationQueue),
	}
	output, err := client.SendMessage(ctx, &message)

	if err != nil {
		return err
	}
	log.Info(logger, *output.MessageId, "Sent SQS message")
	return nil
}

func handleS3EventWithConfig(cfg aws.Config, ctx context.Context, s3Event events.S3Event) error {
	// Configure service clients
	s3svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = env.UsePathStyleS3Opt
	})

	sqssvc := sqs.NewFromConfig(cfg)
	log.Debug(logger, s3svc, "Created S3 client")
	bucket := s3Event.Records[0].S3.Bucket.Name
	log.Debug(logger, bucket, "Reading from bucket")
	uploadedFile := s3Event.Records[0].S3.Object.Key
	log.Info(logger, uploadedFile, "New email file")
	// Get the email body
	resp, err := s3svc.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(uploadedFile),
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return err
	}
	emailBody := buf.String()
	// Parse the URL from the email body
	url, err := parseURLFromEmailBody(emailBody)
	if err != nil {
		return err
	}
	log.Info(logger, url, "Parsed URL from email body")
	// Enqueue the URL for download
	err = enqueueURLForDownload(url, sqssvc, ctx)
	if err != nil {
		return err
	}

	return nil
}
