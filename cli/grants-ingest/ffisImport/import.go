package ffisImport

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/xuri/excelize/v2"
)

func filesToS3Keys(srcDir, dstPrefix, dstDateLayout, dstSuffix string) (map[string]string, error) {
	keyMap := make(map[string]string)
	return keyMap, filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == srcDir {
				return nil
			}
			return fmt.Errorf("found directory at %s but expected a file", path)
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		pubDate, err := findSpreadsheetPublishDate(f)
		if err != nil {
			return err
		}
		keyMap[path] = filepath.Join(dstPrefix, pubDate.Format(dstDateLayout), dstSuffix)
		return nil
	})
}

const (
	publishDateSheet  string = "Sheet1"
	publishDateCell   string = "N2"
	publishDateLayout string = "January 2, 2006"
)

func findSpreadsheetPublishDate(r io.Reader) (t time.Time, err error) {
	xlFile, err := excelize.OpenReader(r)
	if err != nil {
		return
	}
	defer xlFile.Close()

	if cell, err := xlFile.GetCellValue(publishDateSheet, publishDateCell); err == nil {
		return time.Parse(publishDateLayout, cell)
	}
	return
}

func sortKeysByValues(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return m[keys[i]] < m[keys[j]]
	})
	return keys
}

type s3PutObjectAPI interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

func uploadToS3(ctx context.Context, svc s3PutObjectAPI, bucket, src, key string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = svc.PutObject(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(bucket),
		Key:                  aws.String(key),
		Body:                 f,
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	})
	return err
}
