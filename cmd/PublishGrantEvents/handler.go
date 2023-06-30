package main

import (
	"context"
	"encoding/json"
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

type EventBridgePutEventsAPI interface {
	PutEvents(context.Context, *eventbridge.PutEventsInput, ...func(*eventbridge.Options)) (
		*eventbridge.PutEventsOutput, error)
}

func handleEvent(ctx context.Context, pub EventBridgePutEventsAPI, event events.DynamoDBEvent) (events.DynamoDBEventResponse, error) {
	sendMetric("invocation_batch_size", float64(len(event.Records)))

	wg := sync.WaitGroup{}
	wg.Add(len(event.Records))
	failSeq := make(chan string, len(event.Records))
	for i := range event.Records {
		go func(r events.DynamoDBEventRecord) {
			defer wg.Done()
			if err := handleRecord(ctx, pub, r); err != nil {
				failSeq <- r.Change.SequenceNumber
				log.Error(logger, "Failed to handle record in batch", err,
					"sequence_number", r.Change.SequenceNumber)
				sendMetric("record.failed", 1, fmt.Sprintf("event_name:%s", r.EventName))
			}
		}(event.Records[i])
	}
	wg.Wait()
	close(failSeq)

	failures := make([]events.DynamoDBBatchItemFailure, len(failSeq))
	idx := 0
	for seq := range failSeq {
		failures[idx].ItemIdentifier = seq
		idx++
	}
	return events.DynamoDBEventResponse{BatchItemFailures: failures}, nil
}

func handleRecord(ctx context.Context, pub EventBridgePutEventsAPI, rec events.DynamoDBEventRecord) error {
	logger := log.With(logger, "event_name", rec.EventName,
		"keys", rec.Change.Keys, "sequence_number", rec.Change.SequenceNumber)

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
		return log.Errorf(logger, "error publishing to EventBridge", err)
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

	var newVersion, prevVersion *usdr.Grant
	if record.EventName == DDBStreamEventInsert || record.EventName == DDBStreamEventModify {
		metricTag := "change:NewImage"
		logger := log.With(logger, "change", "NewImage")
		image := record.Change.NewImage

		sendMetric("item_image.build", 1, metricTag)
		if grant, err := GuardPanic(NewItemMapper(image).Grant); err != nil {
			sendMetric("item_image.unbuildable", 1, metricTag)
			return nil, log.Errorf(logger, "error building grant from change image", err)
		} else if err := grant.Validate(); err != nil {
			sendMetric("grant_data.invalid", 1, metricTag)
			return nil, log.Errorf(logger, "grant data from ItemMapper is invalid", err)
		} else {
			newVersion = &grant
		}
	}
	if record.EventName == DDBStreamEventModify || record.EventName == DDBStreamEventDelete {
		metricTag := "change:OldImage"
		logger := log.With(logger, "change", "OldImage")
		image := record.Change.OldImage

		sendMetric("item_image.build", 1, metricTag)
		if grant, err := GuardPanic(NewItemMapper(image).Grant); err != nil {
			sendMetric("item_image.unbuildable", 1, metricTag)
			return nil, log.Errorf(logger, "error building grant from change image", err)
		} else {
			prevVersion = &grant
		}
		if err := prevVersion.Validate(); err != nil {
			sendMetric("grant_data.invalid", 1, metricTag)
			log.Warn(logger, "grant data from ItemMapper is invalid", err)
		}
	}

	modificationEvent, err := usdr.NewGrantModificationEvent(newVersion, prevVersion)
	if err != nil {
		return nil, log.Errorf(logger, "Error building event", err)
	}
	if err := modificationEvent.Validate(); err != nil {
		log.Warn(logger, "grant modification event contains invalid data", "error", err)
	}

	data, err := json.Marshal(modificationEvent)
	if err != nil {
		return nil, log.Errorf(logger, "Error marshaling event to JSON", err)
	}
	return data, nil
}
