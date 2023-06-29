package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	"github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/usdr"
)

const (
	DDBStreamEventInsert = string(events.DynamoDBOperationTypeInsert)
	DDBStreamEventModify = string(events.DynamoDBOperationTypeModify)
	DDBStreamEventDelete = string(events.DynamoDBOperationTypeRemove)
)

type PutEventsClient interface {
	PutEvents(context.Context, *eventbridge.PutEventsInput, ...func(*eventbridge.Options)) (
		*eventbridge.PutEventsOutput, error)
}

func handleWithConfig(cfg aws.Config, ctx context.Context, event events.DynamoDBEvent) (events.DynamoDBEventResponse, error) {
	sendMetric("invocation_batch_size", float64(len(event.Records)))
	ebClient := eventbridge.NewFromConfig(cfg)

	wg := sync.WaitGroup{}
	wg.Add(len(event.Records))
	failIds := make(chan string, len(event.Records))
	for i := range event.Records {
		go func(r events.DynamoDBEventRecord) {
			defer wg.Done()
			if err := handleRecord(ctx, ebClient, r); err != nil {
				failIds <- r.Change.SequenceNumber
				log.Error(logger, "Failed to handle record in batch", err,
					"sequence_number", r.Change.SequenceNumber)
				sendMetric("record.failed", 1, fmt.Sprintf("event_name:%s", r.EventName))
			} else {
				sendMetric("record.handled", 1, fmt.Sprintf("event_name:%s", r.EventName))
			}
		}(event.Records[i])
	}
	wg.Wait()
	close(failIds)

	failures := make([]events.DynamoDBBatchItemFailure, len(failIds))
	counter := 0
	for seq := range failIds {
		failures[counter] = events.DynamoDBBatchItemFailure{ItemIdentifier: seq}
		counter++
	}
	return events.DynamoDBEventResponse{BatchItemFailures: failures}, nil
}

func handleRecord(ctx context.Context, pub PutEventsClient, rec events.DynamoDBEventRecord) error {
	eventDetail, err := buildGrantModificationEventJSON(rec)
	if err != nil {
		return err
	}
	if _, err := pub.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{{
			Source:       aws.String("org.usdigitalresponse.grants-ingest"),
			DetailType:   aws.String("GrantModification"),
			Detail:       aws.String(string(eventDetail)),
			Time:         aws.Time(rec.Change.ApproximateCreationDateTime.Time),
			EventBusName: aws.String(env.EventBusName),
		}},
	}); err != nil {
		return log.Errorf(logger, "Failed to publish GrantModificationEvent", err)
	}

	sendMetric("event.published", 1)
	log.Info(logger, "Published GrantModificationEvent")
	return nil
}

func buildGrantModificationEventJSON(record events.DynamoDBEventRecord) ([]byte, error) {
	logger := log.With(logger, "change_size_bytes", record.Change.SizeBytes,
		"change_approximate_creation_time", record.Change.ApproximateCreationDateTime,
		"keys", record.Change.Keys, "sequence_number", record.Change.SequenceNumber,
		"event_id", record.EventID, "event_version", record.EventVersion,
		"event_name", record.EventName,
	)

	var (
		newVersion, prevVersion *usdr.Grant
		buildErr                error
	)
	if record.EventName == DDBStreamEventInsert || record.EventName == DDBStreamEventModify {
		if newVersion, buildErr = buildGrantVersion(record.Change.NewImage); buildErr != nil {
			return nil, log.Errorf(logger, "Error building grant from new image", buildErr)
		}
		if err := newVersion.Validate(); err != nil {
			return nil, log.Errorf(logger, "new grant version is invalid", err)
		}
	}
	if record.EventName == DDBStreamEventModify || record.EventName == DDBStreamEventDelete {
		if prevVersion, buildErr = buildGrantVersion(record.Change.OldImage); buildErr != nil {
			return nil, log.Errorf(logger, "Error building grant from old image", buildErr)
		}
		if err := prevVersion.Validate(); err != nil {
			log.Warn(logger, "previous grant version is invalid", "error", err)
		}
	}

	modificationEvent, err := usdr.NewGrantModificationEvent(newVersion, prevVersion)
	if err != nil {
		return nil, log.Errorf(logger, "Error building event", err)
	}

	data, err := json.Marshal(modificationEvent)
	if err != nil {
		return nil, log.Errorf(logger, "Error marshaling event to JSON", err)
	}
	return data, nil
}

func buildGrantVersion(image map[string]events.DynamoDBAttributeValue) (g *usdr.Grant, err error) {
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case error:
				err = v
			case string:
				err = errors.New(v)
			default:
				err = fmt.Errorf("unknown panic: %+v of type %T", r, r)
			}
		}
	}()
	result := NewItemMapper(image).Grant()
	return &result, err
}
