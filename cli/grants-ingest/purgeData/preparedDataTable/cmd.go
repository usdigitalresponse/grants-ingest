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
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
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
	PurgeFFIS        bool                `help:"Purge all item attributes sourced from FFIS.org data (ignored if --delete-items is given)."`
	PurgeGov         bool                `help:"Purge all item attributes sourced from Grants.gov data (ignored if --delete-items is given)."`
	KeepRevisionIDs  bool                `name:"keep-revision-ids" help:"Retain item revision_id attributes when purging data (ignored if --delete-items is given)."`
	DeleteItems      bool                `help:"Delete all table items completely."`
	ReadConcurrency  ct.ConcurrencyLimit `default:"1" help:"Max DynamoDB parallel scan workers."`
	WriteConcurrency ct.ConcurrencyLimit `default:"10" help:"Max concurrent batch-write operations."`
	TotalsAfter      ct.TotalsAfter      `default:"1000" help:"Log DynamoDB item totals after this many successful/failed deletions (silent if 0)."`
	IgnoreStreams    bool                `help:"Purge data even if the DynamoDB table has an active stream. Not recommended."`
	DryRun           bool                `help:"Dry run only - no DynamoDB table items will be modified or deleted."`

	// Internal
	ctx    context.Context
	stop   context.CancelFunc
	ddb    *dynamodb.Client
	logger *log.Logger
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
	cmd.ddb = dynamodb.NewFromConfig(cfg)
	if !cmd.IgnoreStreams {
		if !cmd.tableStreamsInactive(dynamodbstreams.NewFromConfig(cfg)) {
			app.Errorf("WARNING: Active stream(s) found on the target DynamoDB table.")
			app.Errorf("It is strongly advised that you avoid purging data from a table with an active stream.")
			app.Errorf("Therefore, this command will fail.")
			app.Errorf("Use --ignore-streams to skip this check in the future.")
			kong.NoDefaultHelp().Apply(app)
			return fmt.Errorf("active streams detected on target table")
		}
	} else {
		wait := time.Second * 5
		log.Warn(*cmd.logger, "Skipping DynamoDB stream check. This is NOT recommended!",
			"proceed_after", wait)
		time.Sleep(wait)
	}

	return nil
}

func (cmd *Cmd) Run() error {
	defer cmd.stop()

	scannedItems := make(chan DDBItem)
	batchedRequests := make(chan []types.WriteRequest)
	purgeCounts := make(chan int)

	reportingDone := make(chan struct{})
	go func() {
		defer func() { close(reportingDone) }()
		var totalPurged int64
		for nextCount := range purgeCounts {
			for i := 0; i < nextCount; i++ {
				totalPurged++
				if cmd.TotalsAfter.Check(totalPurged) {
					log.Info(*cmd.logger, "Updated purged items total", "count", totalPurged)
				}
			}
		}
		log.Info(*cmd.logger, "Final count of purged items", "count", totalPurged)
	}()

	go func() {
		defer close(batchedRequests)
		var totalScanned int64
		currentBatch := make([]types.WriteRequest, 0, 25)
		for item := range scannedItems {
			totalScanned++
			if cmd.TotalsAfter.Check(totalScanned) {
				log.Info(*cmd.logger, "Updated scanned items total", "count", totalScanned)
			}
			currentBatch = append(currentBatch, cmd.writeRequestForItem(item))
			if len(currentBatch) == 25 {
				batchedRequests <- currentBatch
				currentBatch = make([]types.WriteRequest, 0, 25)
			}
		}
		if len(currentBatch) > 0 {
			batchedRequests <- currentBatch
		}
	}()

	var purgeItemsErr error
	purgeWg := sync.WaitGroup{}
	for i := 0; i < int(cmd.WriteConcurrency); i++ {
		logger := log.WithSuffix(*cmd.logger, "worker_id", i)
		purgeWg.Add(1)
		go func() {
			defer purgeWg.Done()
			err := cmd.purgeWorker(logger, batchedRequests, purgeCounts)
			if err != nil && err != context.Canceled {
				log.Error(*cmd.logger,
					"Stopping application due to fatal error encountered while purging DynamoDB items",
					err)
				purgeItemsErr = err
				cmd.stop()
			}
		}()
	}

	var scanTableErr error
	scanWg := sync.WaitGroup{}
	for i := 0; i < int(cmd.ReadConcurrency); i++ {
		scanWg.Add(1)
		segmentId := i
		go func() {
			defer scanWg.Done()
			err := cmd.scanTable(segmentId, scannedItems)
			if err != nil && err != context.Canceled {
				log.Error(*cmd.logger,
					"Stopping application due to fatal error encountered while scanning DynamoDB items",
					err)
				scanTableErr = err
				cmd.stop()
			}
		}()
	}

	scanWg.Wait()
	close(scannedItems)
	purgeWg.Wait()
	close(purgeCounts)
	<-reportingDone

	if cmd.ctx.Err() != nil || purgeItemsErr != nil || scanTableErr != nil {
		return fmt.Errorf("the operation completed with errors")
	}

	return nil
}

func (cmd *Cmd) scanTable(segmentId int, ch chan<- DDBItem) (err error) {
	logger := log.WithSuffix(*cmd.logger, "worker_id", segmentId)
	defer func() {
		msg := "Scan worker shutting down"
		if err == nil {
			log.Debug(logger, msg, "reason", "no more work")
		} else if err == context.Canceled {
			log.Warn(logger, msg, "reason", "shutdown requested")
		} else {
			log.Error(logger, msg, err, "reason", "fatal error")
		}
	}()

	input := &dynamodb.ScanInput{
		TableName:      aws.String(cmd.TableName),
		ConsistentRead: aws.Bool(true),
	}
	if cmd.ReadConcurrency > 1 {
		input.Segment = aws.Int32(int32(segmentId))
		input.TotalSegments = aws.Int32(int32(cmd.ReadConcurrency))
	}
	if cmd.DeleteItems {
		input.ProjectionExpression = aws.String("grant_id")
	}

	for {
		select {
		case <-cmd.ctx.Done():
			return cmd.ctx.Err()
		default:
			resp, err := cmd.ddb.Scan(cmd.ctx, input)
			if err != nil {
				log.Error(logger, "Error scanning DynamoDB table items", err)
				return err
			}
			for _, item := range resp.Items {
				log.Debug(logger, "Item found in scan", "item", item)
				ch <- item
			}
			if resp.LastEvaluatedKey == nil {
				return nil
			}
			input.ExclusiveStartKey = resp.LastEvaluatedKey
		}
	}
}

func (cmd *Cmd) purgeWorker(logger log.Logger, batches <-chan []types.WriteRequest, purgeCounts chan<- int) (err error) {
	defer func() {
		msg := "Purge worker shutting down"
		if err == nil {
			log.Debug(logger, msg, "reason", "no more work")
		} else if err == context.Canceled {
			log.Warn(logger, msg, "reason", "shutdown requested")
		} else {
			log.Error(logger, msg, err, "reason", "fatal error")
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
				if err := cmd.purgeItems(logger, requests, purgeCounts); err != nil {
					return err
				}
			case <-cmd.ctx.Done():
				return cmd.ctx.Err()
			}
		}
	}
}

func (cmd *Cmd) purgeItems(logger log.Logger, batch []types.WriteRequest, purgeCounts chan<- int) error {
	input := dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			cmd.TableName: batch,
		},
	}

	err := backoff.RetryNotify(
		func() error {
			thisBatchSize := len(input.RequestItems[cmd.TableName])
			if cmd.DryRun {
				purgeCounts <- thisBatchSize
				return nil
			}
			resp, err := cmd.ddb.BatchWriteItem(cmd.ctx, &input)
			if err != nil {
				return backoff.Permanent(err)
			}
			countUnprocessed := len(resp.UnprocessedItems)
			purgeCounts <- (thisBatchSize - countUnprocessed)
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

func (cmd *Cmd) writeRequestForItem(item DDBItem) types.WriteRequest {
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
			if k == "grant_id" {
				continue
			}
			if k == "Bill" {
				continue
			}
			if k == "revision_id" {
				continue
			}
			delete(item, k)
		}
	}
	log.Debug(*cmd.logger, "Prepared PutRequest item", "item", item)
	req.PutRequest = &types.PutRequest{Item: item}
	return req
}

func (cmd *Cmd) tableStreamsInactive(c *dynamodbstreams.Client) bool {
	resp, err := c.ListStreams(cmd.ctx, &dynamodbstreams.ListStreamsInput{
		TableName: aws.String(cmd.TableName),
		Limit:     aws.Int32(1),
	})
	if err != nil {
		log.Error(*cmd.logger, "Error listing streams for DynamoDB table", err,
			"table_name", cmd.TableName)
		return false
	}
	return len(resp.Streams) == 0
}
