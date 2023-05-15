package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/go-multierror"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	"github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/ffis"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

const (
	MB = int64(1024 * 1024)
)

type opportunity ffis.FFISFundingOpportunity

// S3ObjectKey returns a string to use as the object key when saving the opportunity to an S3 bucket.
func (o *opportunity) S3ObjectKey() string {
	firstThree := strconv.FormatInt(o.GrantID, 10)[:3]
	return fmt.Sprintf("%s/%d/ffis.org/v1.json", firstThree, o.GrantID)
}

// handleS3Event handles events representing S3 bucket notifications of type "ObjectCreated:*"
func handleS3EventWithConfig(cfg aws.Config, ctx context.Context, s3Event events.S3Event) error {
	// Configure service clients
	s3svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = env.UsePathStyleS3Opt
	})

	// Create an opportunities channel to receive opportunities from the source sheet
	opportunities := make(chan opportunity)

	// Create a pool of workers to consume and upload values received from the opportunities channel
	processingSpan, processingCtx := tracer.StartSpanFromContext(ctx, "processing")
	wg := multierror.Group{}
	for i := 0; i < env.MaxConcurrentUploads; i++ {
		wg.Go(func() error {
			return processOpportunities(processingCtx, s3svc, opportunities)
		})
	}

	// Receive the source records from the event and process each spreadsheet within them
	sourcingSpan, sourcingCtx := tracer.StartSpanFromContext(ctx, "handle.records")

	sourcingErrs := &multierror.Error{}
	for i, record := range s3Event.Records {
		recordSpan, recordCtx := tracer.StartSpanFromContext(sourcingCtx, "handle.record")

		sourcingErr := func(i int, record events.S3EventRecord) error {
			sourceBucket := record.S3.Bucket.Name
			sourceKey := record.S3.Object.Key
			logger := log.With(logger, "event_name", record.EventName, "record_index", i,
				"source_bucket", sourceBucket, "source_object_key", sourceKey)

			log.Info(logger, "Downloading ffis.org spreadsheet from S3")

			resp, err := s3svc.GetObject(recordCtx, &s3.GetObjectInput{
				Bucket: aws.String(sourceBucket),
				// The FFIS xlsx spreadsheet
				Key: aws.String(sourceKey),
			})
			if err != nil {
				log.Error(logger, "Error getting source S3 object", err)
				return err
			}

			defer resp.Body.Close()

			log.Info(logger, "Parsing excel file")

			parsedOpportunities, err := parseXLSXFile(resp.Body, logger)

			if err != nil {
				log.Error(logger, "Error parsing excel file: ", err)
				return err
			}

			for _, opp := range parsedOpportunities {
				// Cast opp to opportunity type and send it down the channel
				// for processing
				opportunities <- opportunity(opp)
			}

			return nil
		}(i, record)
		if sourcingErr != nil {
			sourcingErrs = multierror.Append(sourcingErrs, sourcingErr)
		}
		recordSpan.Finish(tracer.WithError(sourcingErr))
	}

	// All source records have been consumed; close the channel so that workers shut down
	// after the channel is emptied.
	close(opportunities)
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

	return nil
}

// processOpportunities consumes opportunities from the channel and uploads them to S3.
func processOpportunities(ctx context.Context, svc *s3.Client, ch <-chan opportunity) (errs error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "processing.worker")

	whenCanceled := func() error {
		err := ctx.Err()
		log.Debug(logger, "Done processing opportunities because context canceled", "reason", err)
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
			case opportunity, ok := <-ch:
				if !ok {
					log.Debug(logger, "Done processing opportunities because channel is closed")
					span.Finish()
					return
				}

				workSpan, ctx := tracer.StartSpanFromContext(ctx, "processing.worker.work")
				err := processOpportunity(ctx, svc, opportunity)
				if err != nil {
					sendMetric("opportunity.failed", 1)
					errs = multierror.Append(errs, err)
				}
				workSpan.Finish(tracer.WithError(err))

			case <-ctx.Done():
				return whenCanceled()
			}
		}
	}
}

// processOpportunity marshals the opportunity to JSON and uploads it to S3.
func processOpportunity(ctx context.Context, svc S3ReadWriteObjectAPI, opp opportunity) error {
	logger := log.With(logger,
		"opportunity_id", opp.GrantID, "opportunity_number", opp.OppNumber)

	key := opp.S3ObjectKey()

	logger = log.With(logger, "bucket", env.DestinationBucket, "key", key)

	log.Info(logger, "Marshaling opportunity to JSON")

	// Convert the parsed opportunity to JSON
	b, err := json.Marshal(opp)
	if err != nil {
		return log.Errorf(logger, "Error marshaling JSON for opportunity", err)
	}

	log.Info(logger, "Uploading opportunity")

	// Upload the object
	if err := UploadS3Object(ctx, svc, env.DestinationBucket, key, bytes.NewReader(b)); err != nil {
		return log.Errorf(logger, "Error uploading prepared opportunity to S3", err)
	}

	log.Info(logger, "Successfully uploaded opportunity")

	sendMetric("opportunity.created", 1)

	return nil
}
