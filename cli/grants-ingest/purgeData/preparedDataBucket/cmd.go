package preparedDataBucket

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/alecthomas/kong"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	ct "github.com/usdigitalresponse/grants-ingest/cli/types"
	"github.com/usdigitalresponse/grants-ingest/internal/awsHelpers"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
)

type Cmd struct {
	// Positional arguments
	S3Bucket string `arg:"" name:"bucket" help:"Prepared data S3 bucket name from which to purge objects."`

	// Flags
	MatchPaths     []FilePathMatcher   `placeholder:"glob" help:"Shell filename pattern (i.e. glob) for matching S3 keys to delete."`
	MatchRegex     []RegexMatcher      `placeholder:"expression" help:"Regex pattern for matching S3 keys to delete."`
	PurgeFFIS      bool                `help:"Delete all FFIS.org S3 objects (after applying --filter-prefix, if given)."`
	PurgeGov       bool                `help:"Delete all Grants.gov S3 objects (after applying --filter-prefix, if given)."`
	PurgeAll       bool                `help:"Delete all S3 objects (after applying --filter-prefix, if given)."`
	FilterPrefix   string              `name:"s3-prefix" default:"" help:"Prevent deleting bucket objects outside this prefix."`
	Concurrency    ct.ConcurrencyLimit `default:"10" help:"Max concurrent batch-delete operations."`
	TotalsAfter    ct.TotalsAfter      `default:"1000" help:"Log S3 object totals after this many successful/failed deletions (silent if 0)."`
	S3UsePathStyle bool                `name:"s3-use-path-style" help:"Use path-style addressing for S3 bucket."`
	DryRun         bool                `help:"Dry run only - no files will be uploaded to S3."`

	// Internal
	ctx      context.Context
	stop     context.CancelFunc
	s3svc    *s3.Client
	matchers []Matcher
	logger   *log.Logger
}

var (
	ErrCompletion = errors.New("the operation completed with errors")
	ErrNoMatchers = errors.New("no match options are configured for purging data")
)

func (cmd *Cmd) BeforeApply(app *kong.Kong, logger *log.Logger) error {
	cmd.ctx, cmd.stop = signal.NotifyContext(context.Background(),
		syscall.SIGHUP, syscall.SIGINT, os.Interrupt)
	cmd.matchers = make([]Matcher, 0)
	cmd.logger = logger
	return nil
}

func (cmd *Cmd) AfterApply(app *kong.Kong) error {
	cfg, err := awsHelpers.GetConfig(cmd.ctx)
	if err != nil {
		err := fmt.Errorf("failed to configure AWS SDK: %w", err)
		if !cmd.DryRun {
			return err
		}
	}
	cmd.s3svc = s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = cmd.S3UsePathStyle })

	if cmd.PurgeAll {
		cmd.matchers = append(cmd.matchers, AllMatcher)
	} else {
		if cmd.PurgeFFIS {
			cmd.matchers = append(cmd.matchers, FFISMatcher)
		}
		if cmd.PurgeGov {
			cmd.matchers = append(cmd.matchers, GrantsGovMatcher)
		}
		for _, m := range cmd.MatchRegex {
			cmd.matchers = append(cmd.matchers, &m)
		}
		for _, m := range cmd.MatchPaths {
			cmd.matchers = append(cmd.matchers, &m)
		}
		if len(cmd.matchers) == 0 {
			return ErrNoMatchers
		}
	}

	return nil
}

func (cmd *Cmd) Run(app *kong.Kong) error {
	defer cmd.stop()

	keysToDelete := make(chan []string)
	failedDeletions := make(chan string)
	successfulDeletions := make(chan string)

	var deleteObjectsErr error
	workWg := sync.WaitGroup{}
	for i := 0; i < int(cmd.Concurrency); i++ {
		workLogger := log.WithSuffix(*cmd.logger, "worker_id", i)
		workWg.Add(1)
		go func() {
			defer workWg.Done()
			err := cmd.deleteObjectsWorker(workLogger, keysToDelete, successfulDeletions, failedDeletions)
			if err != nil && err != context.Canceled {
				log.Error(*cmd.logger,
					"Stopping application due to fatal error encountered while purging S3 objects",
					err)
				deleteObjectsErr = err
				cmd.stop()
			}
		}()
	}

	resultWg := sync.WaitGroup{}
	failedObjectKeys := make([]string, len(failedDeletions))
	resultWg.Add(1)
	go func() {
		defer resultWg.Done()
		var totalFailedDeletions int64
		for f := range failedDeletions {
			failedObjectKeys = append(failedObjectKeys, f)
			totalFailedDeletions++
			if cmd.TotalsAfter.Check(totalFailedDeletions) {
				log.Info(*cmd.logger, "Updated failed deletions total", "count", totalFailedDeletions)
			}
		}
		log.Info(*cmd.logger, "Final count of failed deletions", "count", totalFailedDeletions)
	}()

	resultWg.Add(1)
	go func() {
		defer resultWg.Done()
		var totalDeletedObjects int64
		for range successfulDeletions {
			totalDeletedObjects++
			if cmd.TotalsAfter.Check(totalDeletedObjects) {
				log.Info(*cmd.logger, "Updated deleted objects total", "count", totalDeletedObjects)
			}
		}
		log.Info(*cmd.logger, "Final count of deleted objects", "count", totalDeletedObjects)
	}()

	var listObjectsErr error
	go func() {
		defer close(keysToDelete)
		listObjectsErr = cmd.listObjects(cmd.logger, keysToDelete)
	}()

	workWg.Wait()
	close(failedDeletions)
	close(successfulDeletions)
	resultWg.Wait()

	if cmd.ctx.Err() != nil || listObjectsErr != nil || deleteObjectsErr != nil || len(failedObjectKeys) > 0 {
		if len(failedObjectKeys) > 0 {
			log.Warn(*cmd.logger, "Some objects could not be deleted",
				"count", len(failedObjectKeys), "keys", failedObjectKeys)
		}
		return ErrCompletion
	}

	return nil
}

func (cmd *Cmd) listObjects(logger *log.Logger, ch chan<- []string) error {
	var totalMatchCount int64
	params := &s3.ListObjectsV2Input{
		Bucket:  aws.String(cmd.S3Bucket),
		Prefix:  aws.String(cmd.FilterPrefix),
		MaxKeys: aws.Int32(1000),
	}
	for {
		resp, err := cmd.s3svc.ListObjectsV2(cmd.ctx, params)
		if err != nil {
			log.Error(*cmd.logger, "Error listing S3 objects", err)
			return err
		}

		keys := make([]string, *resp.KeyCount)
		matchCount := 0
		for _, obj := range resp.Contents {
			k := *obj.Key
			for _, m := range cmd.matchers {
				if m.Match(k) {
					log.Debug(*cmd.logger, "Matched S3 object", "key", k, "pattern", m.Pattern())
					keys[matchCount] = k
					matchCount++
					totalMatchCount++
					if cmd.TotalsAfter.Check(totalMatchCount) {
						log.Info(*cmd.logger, "Updated matched objects total", "count", totalMatchCount)
					}
				}
			}
		}
		if matchCount > 0 {
			ch <- keys[:matchCount]
		}

		if resp.NextContinuationToken == nil {
			return nil
		}
		params.ContinuationToken = resp.NextContinuationToken
	}
}

func (cmd *Cmd) deleteObjectsWorker(logger log.Logger, work <-chan []string, deleted, failures chan<- string) (err error) {
	defer func() {
		if err == nil {
			log.Debug(logger, "Worker shutting down", "reason", "no more work")
		} else if err == context.Canceled {
			log.Warn(logger, "Worker shutting down", "reason", "shutdown requested")
		} else {
			log.Error(logger, "Worker shutting down", err, "reason", "fatal error")
		}
	}()

	for {
		select {
		case <-cmd.ctx.Done():
			return cmd.ctx.Err()
		default:
			select {
			case batchKeys, ok := <-work:
				if !ok {
					return nil
				}
				deleteErr := cmd.deleteObjects(logger, batchKeys, deleted, failures)
				if deleteErr != nil {
					return fmt.Errorf("error requesting S3 object deletion: %w", err)
				}
			case <-cmd.ctx.Done():
				return cmd.ctx.Err()
			}
		}
	}
}

func (cmd *Cmd) deleteObjects(logger log.Logger, keys []string, deleted, failures chan<- string) error {
	identifiers := make([]types.ObjectIdentifier, len(keys))
	for i, k := range keys {
		identifiers[i].Key = aws.String(k)
	}

	if cmd.DryRun {
		for _, k := range keys {
			deleted <- k
		}
		return nil
	}

	resp, err := cmd.s3svc.DeleteObjects(cmd.ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(cmd.S3Bucket),
		Delete: &types.Delete{Objects: identifiers},
	})

	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		for _, f := range resp.Errors {
			log.Debug(logger, "Failed to delete S3 object",
				"key", f.Key, "message", f.Message, "code", f.Code)
			failures <- *f.Key
		}
	}()
	go func() {
		defer wg.Done()
		for _, d := range resp.Deleted {
			log.Debug(logger, "Deleted S3 object", "key", d.Key)
			deleted <- *d.Key
		}
	}()
	wg.Wait()

	return nil
}
