package ddHelpers

import (
	"fmt"

	ddlambda "github.com/DataDog/datadog-lambda-go"
)

const ServiceNamespace = "grants_ingest"

var ddLambdaMetricSender = ddlambda.Metric

// NewMetricSender creates a function that wraps calls to ddlambda.Metric in order to provide
// consistent namespacing and tagging of metrics emitted by a Lambda function.
//
// The following example usages are functionally equivalent:
//
//	NewMetricSender("my_lambda_function", "sticky:tag")("my_metric", 1234, "another:tag")
//	ddlambda.SendMetric(fmt.Sprintf("%s.my_lambda_function.some_metric", RootNamespace), 1234, "sticky:tag", "another:tag")
func NewMetricSender(namespace string, defaultTags ...string) func(metric string, value float64, tags ...string) {
	return func(metric string, value float64, tags ...string) {
		ddLambdaMetricSender(
			fmt.Sprintf("%s.%s.%s", ServiceNamespace, namespace, metric),
			value,
			append(defaultTags, tags...)...,
		)
	}
}
