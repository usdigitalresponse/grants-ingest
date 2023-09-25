package preparedDataTable

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/cenkalti/backoff/v4"
	ct "github.com/usdigitalresponse/grants-ingest/cli/types"
	"github.com/usdigitalresponse/grants-ingest/internal/awsHelpers"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
)

// Aliases
type (
	DDBItem map[string]types.AttributeValue
)

type Cmd struct {
	// Positional arguments
	TableName string `arg:"" name:"table" help:"Name of the DynamoDB table from which to purge data."`

	// Flags
	PurgeFFIS       bool                `help:"Purge all item attributes sourced from FFIS.org data (ignored if --delete-items is given)."`
	PurgeGov        bool                `help:"Purge all item attributes sourced from Grants.gov data (ignored if --delete-items is given)."`
	KeepRevisionIDs bool                `name:"keep-revision-ids" help:"Retain item revision_id attributes when purging data (ignored if --delete-items is given)."`
	DeleteItems     bool                `help:"Delete all table items completely."`
	Concurrency     ct.ConcurrencyLimit `default:"10" help:"Max concurrent batch-write operations."`
	TotalsAfter     ct.TotalsAfter      `default:"1000" help:"Log DynamoDB item totals after this many successful/failed deletions (silent if 0)."`
	DryRun          bool                `help:"Dry run only - no DynamoDB table items will be modified or deleted."`

	// Internal
	ctx          context.Context
	stop         context.CancelFunc
	ddb          *dynamodb.Client
	purgeCounter chan int
	logger       *log.Logger
}

func (cmd *Cmd) BeforeApply(app *kong.Kong, logger *log.Logger) error {
	cmd.ctx, cmd.stop = signal.NotifyContext(context.Background(),
		syscall.SIGHUP, syscall.SIGINT, os.Interrupt)
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
	cmd.ddb = dynamodb.NewFromConfig(cfg, func(*dynamodb.Options) {})
	return nil
}

func (cmd *Cmd) Run() error {
	defer cmd.stop()

	scannedItems := make(chan DDBItem)
	batchedRequests := make(chan []types.WriteRequest)

	reportingDone := make(chan struct{})
	go func() {
		defer func() { close(reportingDone) }()
		var totalPurged int64
		for nextCount := range cmd.purgeCounter {
			for i := 0; i < nextCount; i++ {
				totalPurged++
				if cmd.TotalsAfter.Check(totalPurged) {
					log.Info(*cmd.logger, "Updated purged items total", "count", totalPurged)
				}
			}
		}
	}()

	go func() {
		currentBatch := make([]types.WriteRequest, 0)
		for item := range scannedItems {
			currentBatch = append(currentBatch, cmd.prepareItemWriteRequest(item))
			if len(currentBatch) == 25 {
				batchedRequests <- currentBatch
				currentBatch = make([]types.WriteRequest, 0)
			}
		}
		if len(currentBatch) > 0 {
			batchedRequests <- currentBatch
		}
	}()

	var purgeItemsErr error
	workWg := sync.WaitGroup{}
	for i := 0; i < int(cmd.Concurrency); i++ {
		logger := log.WithSuffix(*cmd.logger, "worker_id", i)
		workWg.Add(1)
		go func() {
			defer workWg.Done()
			err := cmd.purgeWorker(logger, batchedRequests)
			if err != nil && err != context.Canceled {
				log.Error(*cmd.logger,
					"Stopping application due to fatal error encountered while purging DynamoDB items",
					err)
				purgeItemsErr = err
				cmd.stop()
			}
		}()
	}

	var findItemsErr error
	go func() {
		defer close(scannedItems)
		findItemsErr = cmd.findItems(scannedItems)
	}()

	workWg.Wait()
	close(cmd.purgeCounter)
	<-reportingDone

	if cmd.ctx.Err() != nil || purgeItemsErr != nil || findItemsErr != nil {
		return fmt.Errorf("the operation completed with errors")
	}

	return nil
}

func (cmd *Cmd) findItems(ch chan<- DDBItem) error {
	input := &dynamodb.ScanInput{TableName: aws.String(cmd.TableName)}
	if cmd.DeleteItems {
		input.ProjectionExpression = aws.String("grant_id")
	}

	var totalScanned int64
	for {
		resp, err := cmd.ddb.Scan(cmd.ctx, input)
		if err != nil {
			log.Error(*cmd.logger, "Error scanning DynamoDB table items", err)
			return err
		}
		for _, item := range resp.Items {
			ch <- item
			totalScanned++
			if cmd.TotalsAfter.Check(totalScanned) {
				log.Info(*cmd.logger, "Updated scanned items total", "count", totalScanned)
			}
		}
		if resp.LastEvaluatedKey == nil {
			return nil
		}
		input.ExclusiveStartKey = resp.LastEvaluatedKey
	}
}

func (cmd *Cmd) purgeWorker(logger log.Logger, batches <-chan []types.WriteRequest) (err error) {
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
			case requests, ok := <-batches:
				if !ok {
					return nil
				}
				if err := cmd.purgeItems(logger, requests); err != nil {
					return err
				}
			case <-cmd.ctx.Done():
				return cmd.ctx.Err()
			}
		}
	}
}

func (cmd *Cmd) purgeItems(logger log.Logger, batch []types.WriteRequest) error {
	input := dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			cmd.TableName: batch,
		},
	}

	err := backoff.RetryNotify(
		func() error {
			thisBatchSize := len(input.RequestItems[cmd.TableName])
			resp, err := cmd.ddb.BatchWriteItem(cmd.ctx, &input)
			if err != nil {
				return backoff.Permanent(err)
			}
			countUnprocessed := len(resp.UnprocessedItems)
			cmd.purgeCounter <- thisBatchSize - countUnprocessed
			if countUnprocessed > 0 {
				input.RequestItems = resp.UnprocessedItems
				return fmt.Errorf("dynamodb batch write operation returned %d unprocessed items",
					len(resp.UnprocessedItems))
			}
			return nil
		},
		func() backoff.BackOff {
			b := backoff.NewExponentialBackOff()
			b.MaxElapsedTime = time.Minute * 2
			return b
		}(),
		func(err error, d time.Duration) {
			log.Debug(logger, "DynamoDB batch write operation throttled",
				"retry_after", d, "error", err)
		},
	)

	return err
}

func (cmd *Cmd) prepareItemWriteRequest(item DDBItem) types.WriteRequest {
	req := types.WriteRequest{}

	if cmd.DeleteItems {
		req.DeleteRequest = &types.DeleteRequest{Key: item}
		return req
	}

	if !cmd.KeepRevisionIDs {
		delete(item, "revision_id")
	}
	if cmd.PurgeFFIS {
		delete(item, "Bill")
	}
	if cmd.PurgeGov {
		for k := range item {
			if k != "Bill" && k != "revision_id" {
				delete(item, k)
			}
		}
	}
	req.PutRequest = &types.PutRequest{Item: item}
	return req
}
