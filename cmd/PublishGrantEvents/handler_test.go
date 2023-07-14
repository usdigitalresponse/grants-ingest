package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	goenv "github.com/Netflix/go-env"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/go-kit/log"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/usdr"
)

func setupLambdaEnvForTesting(t *testing.T) {
	t.Helper()

	// Suppress normal lambda log output
	logger = log.NewNopLogger()

	// Configure environment variables
	err := goenv.Unmarshal(goenv.EnvSet{"EVENT_BUS_NAME": "TestBus"}, &env)
	require.NoError(t, err, "Error configuring lambda environment for testing")
}

func getFixtureItem(t *testing.T, path string) map[string]events.DynamoDBAttributeValue {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	b, err := io.ReadAll(f)
	require.NoError(t, err)
	return bytesToItem(t, b)
}

func mapToItem(t *testing.T, data any) map[string]events.DynamoDBAttributeValue {
	t.Helper()
	b, err := json.Marshal(data)
	require.NoError(t, err)
	return bytesToItem(t, b)
}

func bytesToItem(t *testing.T, data []byte) (item map[string]events.DynamoDBAttributeValue) {
	t.Helper()
	require.NoError(t, json.Unmarshal(data, &item))
	return
}

type mockEventBridgePutEventsAPI struct {
	expectedError error
	params        *eventbridge.PutEventsInput
	callCount     int
	mux           sync.Mutex
}

func (m *mockEventBridgePutEventsAPI) PutEvents(ctx context.Context, p *eventbridge.PutEventsInput, _ ...func(*eventbridge.Options)) (*eventbridge.PutEventsOutput, error) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.callCount += 1
	m.params = p
	return nil, m.expectedError
}

func TestHandleEvent(t *testing.T) {
	setupLambdaEnvForTesting(t)

	event := events.DynamoDBEvent{Records: []events.DynamoDBEventRecord{
		{
			EventName: DDBStreamEventInsert,
			Change: events.DynamoDBStreamRecord{
				NewImage:       getFixtureItem(t, "fixtures/goodItem.json"),
				SequenceNumber: "SucceedAfterInsert",
			},
		},
		{
			EventName: DDBStreamEventDelete,
			Change: events.DynamoDBStreamRecord{
				NewImage:       getFixtureItem(t, "fixtures/goodItem.json"),
				SequenceNumber: "SucceedAfterDelete",
			},
		},
		func() events.DynamoDBEventRecord {
			oldImage := getFixtureItem(t, "fixtures/goodItem.json")
			prevRevId := ulid.Make()
			prevRevId.SetTime(uint64(time.Now().Add(-time.Hour * 24).UnixMilli()))
			delete(oldImage, "Bill")
			newRevId := ulid.Make()
			oldImage["revision"] = events.NewStringAttribute(prevRevId.String())
			newImage := getFixtureItem(t, "fixtures/goodItem.json")
			newImage["revision"] = events.NewStringAttribute(newRevId.String())
			return events.DynamoDBEventRecord{
				EventName: DDBStreamEventModify,
				Change: events.DynamoDBStreamRecord{
					NewImage:       newImage,
					OldImage:       oldImage,
					SequenceNumber: "SucceedAfterModify",
				},
			}
		}(),
		{
			EventName: DDBStreamEventInsert,
			Change: events.DynamoDBStreamRecord{
				NewImage:       mapToItem(t, nil),
				SequenceNumber: "FailAfterInsert",
			},
		},
	}}

	mockEB := &mockEventBridgePutEventsAPI{}
	resp, err := handleEvent(context.Background(), mockEB, event)
	assert.NoError(t, err)
	assert.Len(t, resp.BatchItemFailures, 1)
	assert.Equal(t, "FailAfterInsert", resp.BatchItemFailures[0].ItemIdentifier)
	assert.Equal(t, 3, mockEB.callCount)
}

func TestHandleRecord(t *testing.T) {
	setupLambdaEnvForTesting(t)

	t.Run("UpdateItem: both versions valid", func(t *testing.T) {
		oldImage := getFixtureItem(t, "fixtures/goodItem.json")
		prevRevId := ulid.Make()
		prevRevId.SetTime(uint64(time.Now().Add(-time.Hour * 24).UnixMilli()))
		delete(oldImage, "Bill")
		newRevId := ulid.Make()
		oldImage["revision"] = events.NewStringAttribute(prevRevId.String())
		newImage := getFixtureItem(t, "fixtures/goodItem.json")
		newImage["revision"] = events.NewStringAttribute(newRevId.String())
		record := events.DynamoDBEventRecord{
			EventName: DDBStreamEventModify,
			Change: events.DynamoDBStreamRecord{
				NewImage: newImage,
				OldImage: oldImage,
			},
		}
		mockEB := &mockEventBridgePutEventsAPI{}
		err := handleRecord(context.Background(), mockEB, record)
		assert.NoError(t, err)
		assert.Equal(t, mockEB.callCount, 1)
		var modEvent usdr.GrantModificationEvent
		assert.NoError(t, json.Unmarshal([]byte(*mockEB.params.Entries[0].Detail), &modEvent))
		assert.Equal(t, modEvent.Versions.Previous.Revision.Id, prevRevId)
		assert.Equal(t, modEvent.Versions.New.Revision.Id, newRevId)
		assert.Empty(t, modEvent.Versions.Previous.Bill)
		assert.Equal(t, modEvent.Versions.New.Bill, newImage["Bill"].String())
		assert.Equal(t, modEvent.Type.String(), usdr.EventTypeUpdate)
	})

	t.Run("PutItem: new version valid", func(t *testing.T) {
		for _, tt := range []struct {
			name  string
			ebErr error
		}{
			{"EventBridge success", nil},
			// {"EventBridge failure", errors.New("could not publish")},
		} {
			t.Run(tt.name, func(t *testing.T) {
				newImage := getFixtureItem(t, "fixtures/goodItem.json")
				record := events.DynamoDBEventRecord{
					EventName: DDBStreamEventInsert,
					Change: events.DynamoDBStreamRecord{
						NewImage: newImage,
					},
				}
				mockEB := &mockEventBridgePutEventsAPI{expectedError: tt.ebErr}
				err := handleRecord(context.Background(), mockEB, record)
				if tt.ebErr != nil {
					assert.EqualError(t, err, "error publishing to EventBridge: could not publish")
				} else {
					assert.NoError(t, err)
				}
				assert.Equal(t, mockEB.callCount, 1)
				var modEvent usdr.GrantModificationEvent
				assert.NoError(t, json.Unmarshal([]byte(*mockEB.params.Entries[0].Detail), &modEvent))
				assert.Equal(t, modEvent.Type.String(), usdr.EventTypeCreate)
			})
		}
	})

	t.Run("PutItem: new version invalid", func(t *testing.T) {
		newImage := mapToItem(t, nil)
		record := events.DynamoDBEventRecord{
			EventName: DDBStreamEventInsert,
			Change: events.DynamoDBStreamRecord{
				NewImage: newImage,
			},
		}
		mockEB := &mockEventBridgePutEventsAPI{}
		err := handleRecord(context.Background(), mockEB, record)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "grant data from ItemMapper is invalid")
		assert.Equal(t, mockEB.callCount, 0)
	})
}
