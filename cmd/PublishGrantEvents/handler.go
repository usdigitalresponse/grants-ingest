package main

import (
	"context"
	"encoding/json"
	"fmt"

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
	failures := make([]events.DynamoDBBatchItemFailure, 0)

	for _, record := range event.Records {
		if err := handleRecord(ctx, pub, record); err != nil {
			seq := record.Change.SequenceNumber
			log.Error(logger, "Failed to handle record in batch", err, "sequence_number", seq)
			sendMetric("record.failed", 1, fmt.Sprintf("event_name:%s", record.EventName))
			failures = append(failures, events.DynamoDBBatchItemFailure{ItemIdentifier: seq})
			break
		}
	}

	return events.DynamoDBEventResponse{BatchItemFailures: failures}, nil
}

func handleRecord(ctx context.Context, pub EventBridgePutEventsAPI, rec events.DynamoDBEventRecord) error {
	logger := log.With(logger, "ddb_event_name", rec.EventName,
		"ddb_keys", rec.Change.Keys, "ddb_sequence_number", rec.Change.SequenceNumber)

	eventJSON, eventType, err := buildGrantModificationEventJSON(rec)
	if err != nil {
		return err
	}
	logger = log.With(logger, "event_type", eventType)

	eventInput := types.PutEventsRequestEntry{
		Source:       aws.String("org.usdigitalresponse.grants-ingest"),
		DetailType:   aws.String("GrantModificationEvent"),
		Detail:       aws.String(string(eventJSON)),
		Time:         aws.Time(rec.Change.ApproximateCreationDateTime.Time),
		EventBusName: aws.String(env.EventBusName),
	}
	log.Debug(logger, "Publishing to EventBridge",
		"event_bus_name", eventInput.EventBusName, "event_time", eventInput.Time,
		"event_source", eventInput.Source, "event_detail_type", eventInput.DetailType,
		"event_detail", eventInput.Detail, "event_detail_bytes", eventJSON)
	if _, err := pub.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{eventInput},
	}); err != nil {
		return log.Errorf(logger, "error publishing to EventBridge", err)
	}

	sendMetric("event.published", 1, fmt.Sprintf("type:%s", eventType))
	log.Info(logger, "Published GrantModificationEvent")
	return nil
}

func buildGrantModificationEventJSON(record events.DynamoDBEventRecord) ([]byte, string, error) {
	logger := log.With(logger, "ddb_change_size_bytes", record.Change.SizeBytes,
		"ddb_change_approximate_creation_time", record.Change.ApproximateCreationDateTime,
		"ddb_keys", record.Change.Keys, "ddb_sequence_number", record.Change.SequenceNumber,
		"ddb_event_id", record.EventID, "ddb_event_version", record.EventVersion,
		"ddb_event_name", record.EventName,
	)

	var newVersion, prevVersion *usdr.Grant
	if record.EventName == DDBStreamEventInsert || record.EventName == DDBStreamEventModify {
		metricTag := "change:NewImage"
		logger := log.With(logger, "change", "NewImage")
		image := record.Change.NewImage

		sendMetric("item_image.build", 1, metricTag)
		if grant, err := GuardPanic(NewItemMapper(image).Grant); err != nil {
			sendMetric("item_image.unbuildable", 1, metricTag)
			return nil, "", log.Errorf(logger, "error building grant from change image", err)
		} else if err := grant.Validate(); err != nil {
			sendMetric("grant_data.invalid", 1, metricTag)
			return nil, "", log.Errorf(logger, "grant data from ItemMapper is invalid", err)
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
			return nil, "", log.Errorf(logger, "error building grant from change image", err)
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
		return nil, "", log.Errorf(logger, "Error building event", err)
	}
	logger = log.With(logger, "modification_event_type", modificationEvent.Type.String())
	if err := modificationEvent.Validate(); err != nil {
		log.Warn(logger, "grant modification event contains invalid data", "error", err)
	}

	data, err := json.Marshal(modificationEvent)
	if err != nil {
		err = log.Errorf(logger, "Error marshaling event to JSON", err)
	}
	return data, modificationEvent.Type.String(), err
}
