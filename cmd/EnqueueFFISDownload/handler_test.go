package main

import (
	"os"
	"testing"

	"github.com/go-kit/log"
)

func TestParseEmailURL(t *testing.T) {
	logger = log.NewNopLogger()

	var tests = []struct {
		emailPath, expectedURL string
	}{
		{"good.eml", "https://mcusercontent.com/123456/files/file-01.xlsx"},
	}

	for _, test := range tests {
		file, err := os.ReadFile("./fixtures/" + test.emailPath)
		if err != nil {
			t.Errorf("Error opening file: %v", err)
		}
		email := string(file)
		url, err := parseURLFromEmailBody(email)
		if err != nil {
			t.Errorf("Error parsing email body: %v", err)
		}
		if url != test.expectedURL {
			t.Errorf("Expected URL %v, got %v", test.expectedURL, url)
		}

	}

}
