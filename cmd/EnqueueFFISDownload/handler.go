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

func handleS3EventWithConfig(cfg aws.Config, ctx context.Context, s3Event events.S3Event) error {
	// Configure service clients
	s3svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = env.UsePathStyleS3Opt
	})
	log.Debug(logger, s3svc, "Created S3 client")
	return nil
}
