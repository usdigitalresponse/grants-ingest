package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-kit/log"
	"github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/ffis"
)

type MockS3 struct {
	content       []byte
	key           string
	responseError error
}

func (mockS3 *MockS3) Upload(ctx context.Context,
	params *s3.PutObjectInput,
	optFns ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(params.Body)
	mockS3.content = buf.Bytes()
	params.Body.Read(mockS3.content)
	mockS3.key = *params.Key
	return &s3manager.UploadOutput{}, mockS3.responseError
}

type MockHTTP struct {
	testContent   []byte
	responseError error
	statusCode    int
}

func (mockHTTP *MockHTTP) Do(req *http.Request) (resp *http.Response, err error) {
	bodyReaderClose := io.NopCloser(bytes.NewReader(mockHTTP.testContent))
	mockStatusCode := http.StatusOK
	if mockHTTP.statusCode != 0 {
		mockStatusCode = mockHTTP.statusCode
	}
	return &http.Response{Body: bodyReaderClose, StatusCode: mockStatusCode}, mockHTTP.responseError
}

func TestHandleSQSEvent(t *testing.T) {
	logger = log.NewNopLogger()
	env.MaxDownloadBackoff = 100 * time.Millisecond
	var tests = []struct {
		name               string
		message            ffis.FFISMessageDownload
		httpError, s3Error error
		httpStatusCode     int
	}{
		{name: "basic happy path",
			message: ffis.FFISMessageDownload{DownloadURL: "https://www.example.com", SourceFileKey: "/sources/2023/05/01/ffis/raw.eml"}},
		{name: "fails to download file",
			message:   ffis.FFISMessageDownload{DownloadURL: "https://www.example.com", SourceFileKey: "/sources/2023/05/01/ffis/raw.eml"},
			httpError: fmt.Errorf("Error downloading files")},
		{name: "s3 upload fails",
			message: ffis.FFISMessageDownload{DownloadURL: "https://www.example.com", SourceFileKey: "/sources/2023/05/01/ffis/raw.eml"},
			s3Error: fmt.Errorf("Error uploading files")},
		{name: "HTTP error",
			message:        ffis.FFISMessageDownload{DownloadURL: "https://www.example.com", SourceFileKey: "/sources/2023/05/01/ffis/raw.eml"},
			httpStatusCode: 500},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			msgJson, _ := json.Marshal(test.message)
			sqsEvent := events.SQSEvent{
				Records: []events.SQSMessage{
					{
						Body: string(msgJson),
					},
				},
			}
			mockUploader := &MockS3{responseError: test.s3Error}
			mockHTTP := &MockHTTP{testContent: []byte("test content"), responseError: test.httpError, statusCode: test.httpStatusCode}
			err := handleSQSEvent(context.Background(), sqsEvent, mockUploader, mockHTTP)
			// check error content
			if test.httpError == nil && test.s3Error == nil && test.httpStatusCode <= 200 {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				uploadedContent := string(mockUploader.content)
				if uploadedContent != string(mockHTTP.testContent) {
					t.Errorf("Expected %v, got %v", mockHTTP.testContent, uploadedContent)
				}
				expectedKey := strings.Replace(test.message.SourceFileKey, "raw.eml", "download.xlsx", 1)
				if mockUploader.key != expectedKey {
					t.Errorf("Expected %v, got %v", expectedKey, mockUploader.key)
				}
			}
			if test.httpError != nil {
				if !errorContains(err, test.httpError) {
					t.Errorf("Expected error %v, got %v", test.httpError, err)
				}
			}
			if test.s3Error != nil {
				if !errorContains(err, test.s3Error) {
					t.Errorf("Expected error %v, got %v", test.s3Error, err)
				}
			}
			if test.httpStatusCode > 200 {
				if !errorContains(err, ErrDownloadFailed) {
					t.Errorf("Expected error %v, got %v", ErrDownloadFailed, err)
				}
			}
		})
	}
}

func errorContains(actual error, expected error) bool {
	return strings.Contains(actual.Error(), expected.Error())
}
