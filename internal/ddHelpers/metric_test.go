package ddHelpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMetricSender(t *testing.T) {
	restoreMetricSender := ddLambdaMetricSender
	t.Cleanup(func() { ddLambdaMetricSender = restoreMetricSender })
	lastCallArgs := struct {
		metric string
		value  float64
		tags   []string
	}{}
	mockMetricSender := func(metric string, value float64, tags ...string) {
		lastCallArgs.metric = metric
		lastCallArgs.value = value
		lastCallArgs.tags = tags
	}
	ddLambdaMetricSender = mockMetricSender

	sendMetric := NewMetricSender("testing", "foo:bar", "biz:baz")
	sendMetric("my_metric", 1234, "fizz:fuzz", "bizz:buzz")
	assert.Equal(t, "grants_ingest.testing.my_metric", lastCallArgs.metric)
	assert.Equal(t, float64(1234), lastCallArgs.value)
	assert.ElementsMatch(t, []string{"foo:bar", "biz:baz", "fizz:fuzz", "bizz:buzz"}, lastCallArgs.tags)

	sendMetric("another.metric", 43.21, "different_tag:value")
	assert.Equal(t, "grants_ingest.testing.another.metric", lastCallArgs.metric)
	assert.Equal(t, 43.21, lastCallArgs.value)
	assert.ElementsMatch(t, []string{"foo:bar", "biz:baz", "different_tag:value"}, lastCallArgs.tags)
}
