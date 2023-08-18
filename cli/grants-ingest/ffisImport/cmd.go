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
	// Positional arguments
	SourceDirectory string `arg:"" name:"directory" type:"existingdir" help:"Source directory containing FFIS spreadsheets"`
	S3Bucket        string `arg:"" name:"bucket" help:"Destination S3 bucket name"`

	// Flags
	S3Prefix       string `name:"s3-prefix" help:"Path prefix for mapped S3 keys" default:"sources"`
	S3DateLayout   string `name:"s3-date-layout" help:"Date layout for mapped S3 keys" default:"2006/01/02"`
	S3Suffix       string `name:"s3-suffix" help:"Path suffix for mapped S3 keys" default:"ffis.org/download.xlsx"`
	S3UsePathStyle bool   `name:"s3-use-path-style" help:"Use path-style addressing for S3 bucket"`
	DryRun         bool   `help:"Dry run only - no files will be uploaded to S3"`
}

func (cmd *Cmd) Help() string {
	return fmt.Sprintf(`
The provided <directory> is expected to only contain FFIS spreadsheets; filenames may be arbitrary.
Each spreadsheet is read in order to determine its publishing date (which is assumed to be embedded
at a discrete cell location of "%s:%s"), and is sequentially uploaded in earliest-published order.
The destination S3 key is determined by path-joining the S3 prefix, date layout, and suffix,
respectively (by default, "sources/YYYY/MM/DD/ffis.org/download.xlsx"). The date-based components of
the S3 key are determined by the publishing date.`,
		publishDateSheet, publishDateCell)
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

	log.Debug(*logger, "Mapping files in directory to S3 keys...", "directory", cmd.SourceDirectory)
	srcToDst, err := filesToS3Keys(cmd.SourceDirectory, cmd.S3Prefix, cmd.S3DateLayout, cmd.S3Suffix)
	if err != nil {
		return log.Errorf(*logger, "error mapping archive files to S3 keys", err)
	}
	log.Debug(*logger, "Mapped files for upload", "count", len(srcToDst))

	s3svc := s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = cmd.S3UsePathStyle })
	for i, src := range sortKeysByValues(srcToDst) {
		dst := srcToDst[src]
		logger := log.WithSuffix(*logger,
			"source", src, "destination", fmt.Sprintf("s3://%s/%s", cmd.S3Bucket, dst),
			"progress", fmt.Sprintf("%d of %d", i+1, len(srcToDst)))
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
