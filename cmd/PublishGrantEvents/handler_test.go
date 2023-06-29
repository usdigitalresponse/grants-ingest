package main

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	goenv "github.com/Netflix/go-env"
	"github.com/aws/aws-lambda-go/events"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestBuildGrantVersion(t *testing.T) {
	setupLambdaEnvForTesting(t)

	t.Run("build success", func(t *testing.T) {
		item := getFixtureItem(t, "fixtures/item.json")
		require.NotPanics(t, func() { NewItemMapper(item).Grant() })
		_, err := buildGrantVersion(item)
		assert.NoError(t, err)
	})

	t.Run("build failure", func(t *testing.T) {
		item := mapToItem(t, map[string]string{})
		grant, err := buildGrantVersion(item)
		assert.Error(t, err)
		assert.Equal(t, "1234", grant.Opportunity.Id)
	})
}
