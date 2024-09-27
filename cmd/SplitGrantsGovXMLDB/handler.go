package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"io"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-kit/log/level"
	"github.com/hashicorp/go-multierror"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

const (
	MB                         = int64(1024 * 1024)
	GRANT_OPPORTUNITY_XML_NAME = "OpportunitySynopsisDetail_1_0"
	GRANT_FORECAST_XML_NAME    = "OpportunityForecastDetail_1_0"
)

// handleS3Event handles events representing S3 bucket notifications of type "ObjectCreated:*"
// for XML DB extracts saved from Grants.gov. The XML data from the source S3 object provided
// by each event record is read from S3. Grant opportunity/forecast records are extracted from the XML
// and uploaded to a "prepared data" destination bucket as individual S3 objects.
// Uploads are handled by a pool of workers; the size of the pool is determined by the
// MAX_CONCURRENT_UPLOADS environment variable.
// Returns and error that represents any and all errors accumulated during the invocation,
// either while handling a source object or while processing its contents; an error may indicate
// a partial or complete invocation failure.
// Returns nil when all grant records are successfully processed from all source records,
// indicating complete success.
func handleS3Event(ctx context.Context, s3svc *s3.Client, ddbsvc DynamoDBGetItemAPI, s3Event events.S3Event) error {
	// Create a records channel to direct opportunity/forecast values parsed from the source
	// record to individual S3 object uploads
	records := make(chan grantRecord)

	// Create a pool of workers to consume and upload values received from the records channel
	processingSpan, processingCtx := tracer.StartSpanFromContext(ctx, "processing")
	wg := multierror.Group{}
	for i := 0; i < env.MaxConcurrentUploads; i++ {
		wg.Go(func() error {
			return processRecords(processingCtx, s3svc, ddbsvc, records)
		})
	}

	// Iterate over all received source records to split into per-grant values and submit them to
	// the records channel for processing by the workers pool. Instead of failing on the
	// first encountered error, we instead accumulate them into a single "multi-error".
	// Only one source record is consumed at a time; in normal cases, the invocation event
	// will only provide a single source record.
	sourcingSpan, sourcingCtx := tracer.StartSpanFromContext(ctx, "handle.records")
	sourcingErrs := &multierror.Error{}
	for i, record := range s3Event.Records {
		recordSpan, recordCtx := tracer.StartSpanFromContext(sourcingCtx, "handle.record")
		sourcingErr := func(i int, record events.S3EventRecord) error {
			sourceBucket := record.S3.Bucket.Name
			sourceKey := record.S3.Object.Key
			logger := log.With(logger, "event_name", record.EventName, "record_index", i,
				"source_bucket", sourceBucket, "source_object_key", sourceKey)
			log.Info(logger, "Splitting Grants.gov DB extract XML object from S3")

			resp, err := s3svc.GetObject(recordCtx, &s3.GetObjectInput{
				Bucket: aws.String(sourceBucket),
				Key:    aws.String(sourceKey),
			})
			if err != nil {
				log.Error(logger, "Error getting source S3 object", err)
				return err
			}

			buffer := bufio.NewReaderSize(resp.Body, int(env.DownloadChunkLimit*MB))
			if err := readRecords(recordCtx, buffer, records); err != nil {
				log.Error(logger, "Error reading source records from S3", err)
				return err
			}

			log.Info(logger, "Finished splitting Grants.gov DB extract XML")
			return nil
		}(i, record)
		if sourcingErr != nil {
			sourcingErrs = multierror.Append(sourcingErrs, sourcingErr)
		}
		recordSpan.Finish(tracer.WithError(sourcingErr))
	}

	// All source records have been consumed; close the channel so that workers shut down
	// after the channel is emptied.
	close(records)
	sourcingSpan.Finish()

	// Wait for workers to finish processing and collect any errors they encountered
	processingErrs := wg.Wait()
	processingSpan.Finish()

	// Combine any sourcing and processing errors to return as a single "mega-multi-error"
	errs := multierror.Append(sourcingErrs, processingErrs)
	if err := errs.ErrorOrNil(); err != nil {
		var countSourcingErrors, countProcessingErrors int
		if sourcingErrs != nil {
			countSourcingErrors = sourcingErrs.Len()
		}
		if processingErrs != nil {
			countProcessingErrors = processingErrs.Len()
		}
		log.Warn(logger, "Failures occurred during invocation; check logs for details",
			"count_sourcing_errors", countSourcingErrors,
			"count_processing_errors", countProcessingErrors,
			"count_total", errs.Len())
		return err
	}

	// Hooray, no errors!
	return nil
}

// readRecords reads XML from r, sending all parsed grantRecords to ch.
// Returns nil when the end of the file is reached.
// readRecords stops and returns an error when the context is canceled
// or an error is encountered while reading.
func readRecords(ctx context.Context, r io.Reader, ch chan<- grantRecord) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "read.xml")

	// Count records sent to ch
	countSentRecords := 0

	d := xml.NewDecoder(r)
	for {
		// Check for context cancelation before/between reads
		if err := ctx.Err(); err != nil {
			log.Warn(logger, "Context canceled before reading was complete", "reason", err)
			span.Finish(tracer.WithError(err))
			return err
		}

		// End early if we have reached any configured limit on the number of records sent to ch
		if env.MaxSplitRecords > -1 && countSentRecords >= env.MaxSplitRecords {
			break
		}

		token, err := d.Token()
		if err != nil {
			if err == io.EOF {
				// EOF means that we're done reading
				break
			}
			level.Error(logger).Log("msg", "Error reading XML token", "error", err)
			span.Finish(tracer.WithError(err))
			return err
		}

		// When reading the start of a new element, check if it is a grant opportunity or forecast
		if se, ok := token.(xml.StartElement); ok {
			var err error
			if se.Name.Local == GRANT_OPPORTUNITY_XML_NAME {
				var o opportunity
				if err = d.DecodeElement(&o, &se); err == nil {
					countSentRecords++
					ch <- &o
				}
			} else if se.Name.Local == GRANT_FORECAST_XML_NAME && env.IsForecastedGrantsEnabled {
				var f forecast
				if err = d.DecodeElement(&f, &se); err == nil {
					countSentRecords++
					ch <- &f
				}
			}

			if err != nil {
				log.Error(logger, "Error decoding XML", err, "element_name", se.Name.Local)
				span.Finish(tracer.WithError(err))
				return err
			}
		}
	}
	log.Info(logger, "Finished reading source XML")
	span.Finish()
	return nil
}

// processRecords is a work loop that receives and processes grantRecord values until
// the receive channel is closed and returns or the context is canceled.
// It returns a multi-error containing any errors encountered while processing a received
// grantRecord as well as the reason for the context cancelation, if any.
// Returns nil if all records were processed successfully until the channel was closed.
func processRecords(ctx context.Context, s3svc *s3.Client, ddbsvc DynamoDBGetItemAPI, ch <-chan grantRecord) (errs error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "processing.worker")

	whenCanceled := func() error {
		err := ctx.Err()
		log.Debug(logger, "Done processing records because context canceled", "reason", err)
		span.Finish(tracer.WithError(err))
		errs = multierror.Append(errs, err)
		return errs
	}

	// Since channel selection is pseudo-random, this loop runs a preliminary check for
	// canceled context on each iteration to ensure that cancelation is prioritized.
	for {
		select {
		case <-ctx.Done():
			return whenCanceled()

		default:
			select {
			case record, ok := <-ch:
				if !ok {
					log.Debug(logger, "Done processing records because channel is closed")
					span.Finish()
					return
				}

				workSpan, ctx := tracer.StartSpanFromContext(ctx, "processing.worker.work")
				err := processRecord(ctx, s3svc, ddbsvc, record)
				if err != nil {
					sendMetric("record.failed", 1)
					errs = multierror.Append(errs, err)
				}
				workSpan.Finish(tracer.WithError(err))

			case <-ctx.Done():
				return whenCanceled()
			}
		}
	}
}

// processRecord takes a single record and conditionally uploads an XML representation
// of the grant forecast/opportunity to its configured S3 destination.
// Before uploading, the last-modified date of a matching extant DynamoDB item (if any)
// is compared with the last-modified date the record on-hand.
// An upload is initiated when the record on-hand has a last-modified date that is more recent
// than that of the extant item, or when no extant item exists.
func processRecord(ctx context.Context, s3svc S3PutObjectAPI, ddbsvc DynamoDBGetItemAPI, record grantRecord) error {
	logger := record.logWith(logger)

	lastModified, err := record.lastModified()
	if err != nil {
		return log.Errorf(logger, "Error getting last modified time for record", err)
	}
	logger = log.With(logger, "record_last_modified", lastModified)
	log.Debug(logger, "Parsed last modified time from record last update date")

	key := record.s3ObjectKey()
	logger = log.With(logger, "table", env.DynamoDBTableName, "bucket", env.DestinationBucket, "key", key)
	remoteLastModified, err := GetDynamoDBLastModified(ctx, ddbsvc, env.DynamoDBTableName, record.dynamoDBItemKey())
	if err != nil {
		return log.Errorf(logger, "Error determining last modified time for remote record", err)
	}
	logger = log.With(logger, "remote_last_modified", remoteLastModified)

	isNew := false
	if remoteLastModified != nil {
		if !remoteLastModified.Before(lastModified) {
			log.Debug(logger, "Skipping record upload because the extant record is up-to-date")
			sendMetric("record.skipped", 1)
			return nil
		}
		log.Debug(logger, "Uploading updated record to replace outdated remote record")
	} else {
		isNew = true
		log.Debug(logger, "Uploading new record")
	}

	b, err := record.toXML()
	if err != nil {
		return log.Errorf(logger, "Error marshaling XML for record", err)
	}

	if err := UploadS3Object(ctx, s3svc, env.DestinationBucket, key, bytes.NewReader(b)); err != nil {
		return log.Errorf(logger, "Error uploading prepared grant record to S3", err)
	}

	log.Info(logger, "Successfully uploaded record")
	if isNew {
		sendMetric("record.created", 1)
	} else {
		sendMetric("record.updated", 1)
	}
	return nil
}
