package main

import (
	"context"
	"encoding/xml"
	"errors"
	"io"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/go-multierror"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// handleS3Event handles events representing S3 bucket notifications of type "ObjectCreated:*"
// for XML DB extracts saved from Grants.gov and split into separate files via the SplitGrantsGovXMLDB Lambda.
// The XML data from the source S3 object provided represents an individual grant opportunity.
// Returns an error that represents any and all errors accumulated during the invocation,
// either while handling a source object or while processing its contents; an error may indicate
// a partial or complete invocation failure.
// Returns nil when all grant opportunities are successfully processed from all source records,
// indicating complete success.
func handleS3EventWithConfig(s3svc *s3.Client, dynamodbsvc DynamoDBUpdateItemAPI, ctx context.Context, s3Event events.S3Event) error {
	wg := multierror.Group{}
	for _, record := range s3Event.Records {
		func(record events.S3EventRecord) {
			wg.Go(func() (err error) {
				span, ctx := tracer.StartSpanFromContext(ctx, "handle.record")
				defer span.Finish(tracer.WithError(err))
				defer func() {
					if err != nil {
						sendMetric("record.failed", 1)
					}
				}()

				sourceBucket := record.S3.Bucket.Name
				sourceKey := record.S3.Object.Key
				logger := log.With(logger, "event_name", record.EventName,
					"source_bucket", sourceBucket, "source_object_key", sourceKey)

				resp, err := s3svc.GetObject(ctx, &s3.GetObjectInput{
					Bucket: aws.String(sourceBucket),
					Key:    aws.String(sourceKey),
				})
				if err != nil {
					log.Error(logger, "Error getting source S3 object", err)
					return err
				}

				record, err := decodeNextGrantRecord(resp.Body)
				resp.Body.Close()
				if err != nil {
					log.Error(logger, "Error decoding S3 object XML to record", err)
					return err
				}
				return processGrantRecord(ctx, dynamodbsvc, record)
			})
		}(record)
	}

	errs := wg.Wait()
	if err := errs.ErrorOrNil(); err != nil {
		log.Warn(logger, "Failures occurred during invocation; check logs for details",
			"count_errors", errs.Len(),
			"count_s3_events", len(s3Event.Records))
		return err
	}
	return nil
}

// decodeNextGrantRecord reads XML from r until a grantRecord can be decoded or EOF is reached.
// It stops reading and returns the grantRecord as soon as one is decoded.
// If there is an error reading or unmarshalling XML, it returns a nil grantRecord and the encountered error.
func decodeNextGrantRecord(r io.Reader) (grantRecord, error) {
	d := xml.NewDecoder(r)
	for {
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		if se, ok := token.(xml.StartElement); ok {
			if se.Name.Local == "OpportunitySynopsisDetail_1_0" {
				var o opportunity
				err := d.DecodeElement(&o, &se)
				return o, err
			}
			if se.Name.Local == "OpportunityForecastDetail_1_0" {
				f := forecast{}
				err := d.DecodeElement(&f, &se)
				return f, err
			}
		}
	}
}

// processOpportunity takes a single opportunity and uploads an XML representation of the
// opportunity to its configured DynamoDB table.
func processGrantRecord(ctx context.Context, svc DynamoDBUpdateItemAPI, rec grantRecord) error {
	logger := rec.logWith(logger)

	itemAttrs, err := rec.dynamoDBAttributeMap()
	if err != nil {
		return log.Errorf(logger, "Error marshaling grantRecord to DynamoDB attributes map", err)
	}
	if err := UpdateDynamoDBItem(ctx, svc, env.DestinationTable, rec.dynamoDBItemKey(), itemAttrs); err != nil {
		var conditionalCheckErr *types.ConditionalCheckFailedException
		if errors.As(err, &conditionalCheckErr) {
			log.Warn(logger, "Grants.gov data already matches the target DynamoDB item",
				"error", conditionalCheckErr)
			return nil
		}
		return log.Errorf(logger, "Error uploading prepared grant opportunity to DynamoDB", err)
	}

	log.Info(logger, "Successfully uploaded opportunity")
	sendMetric("record.saved", 1)
	return nil
}
