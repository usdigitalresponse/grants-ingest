package ffisImport

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/usdigitalresponse/grants-ingest/internal/awsHelpers"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
)

type Cmd struct {
	Directory      string `short:"d" type:"existingdir" help:"Directory containing FFIS spreadsheets" required:""`
	S3Bucket       string `short:"b" name:"s3-bucket" help:"Name of the S3 bucket" required:""`
	S3Prefix       string `name:"s3-prefix" help:"Path prefix for mapped S3 keys" default:"sources"`
	S3DateLayout   string `name:"s3-date-layout" help:"Date layout for mapped S3 keys" default:"2006/01/02"`
	S3Suffix       string `name:"s3-suffix" help:"Path suffix for mapped S3 keys" default:"ffis.org/download.xlsx"`
	S3UsePathStyle bool   `name:"s3-use-path-style" help:"Use path-style addressing for S3 buckets"`
	DryRun         bool   `help:"Dry run only - no files will be uploaded to S3"`
}

func (cmd *Cmd) Run(app *kong.Kong, logger *log.Logger) error {
	ctx := context.Background()
	cfg, err := awsHelpers.GetConfig(ctx)
	if err != nil {
		err := fmt.Errorf("failed to configure AWS SDK: %w", err)
		if !cmd.DryRun {
			return err
		}
		app.Errorf(err.Error())
	}

	log.Debug(*logger, "Mapping files in directory to S3 keys...", "directory", cmd.Directory)
	srcToDst, err := filesToS3Keys(cmd.Directory, cmd.S3Prefix, cmd.S3DateLayout, cmd.S3Suffix)
	if err != nil {
		return log.Errorf(*logger, "error mapping archive files to S3 keys", err)
	}

	s3svc := s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = cmd.S3UsePathStyle })
	for _, src := range sortKeysByValues(srcToDst) {
		dst := srcToDst[src]
		logger := log.WithSuffix(*logger,
			"source", src, "destination", fmt.Sprintf("s3://%s/%s", cmd.S3Bucket, dst))
		log.Debug(logger, "Uploading file to S3")
		if !cmd.DryRun {
			if err := uploadToS3(ctx, s3svc, cmd.S3Bucket, src, dst); err != nil {
				return log.Errorf(logger, "Source file failed to upload", err)
			}
		}
		log.Info(logger, "Uploaded file to S3")
	}

	return nil
}
