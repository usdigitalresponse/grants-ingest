locals {
  datadog_metric_metadata_input = {
    for k, v in var.datadog_metrics_metadata :
    startswith(k, "grants_ingest.") ? k : "grants_ingest.${k}" => v
  }
  datadog_metric_metadata_exists = {
    for k in compact([
      for k in keys(data.http.datadog_metric_metadata) :
      (data.http.datadog_metric_metadata[k].status_code == 200 ? k : "")
    ]) : k => local.datadog_metric_metadata_input[k]
  }
}

/*
This data source allows us to check which subset of metrics in local.datadog_metric_metadata_input
are currently registered in Datadog (produce a 200 response).
*/
data "http" "datadog_metric_metadata" {
  for_each = local.datadog_metric_metadata_input

  url    = "https://api.datadoghq.com/api/v1/metrics/${each.key}"
  method = "GET"

  request_headers = {
    Accept               = "application/json"
    "DD-API-KEY"         = var.datadog_api_key
    "DD-APPLICATION-KEY" = var.datadog_app_key
  }
}

/*
The datadog_metric_metadata resource only works when the underlying metric already exists.
Therefore, these resources are dynamically provisioned according to the intersection of metrics
that are: 1) defined in locals.datadog_metric_metadata_input, and 2) exist in Datadog.
*/
resource "datadog_metric_metadata" "custom" {
  for_each = local.datadog_metric_metadata_exists

  metric      = each.key
  short_name  = each.value.short_name
  description = each.value.description
  unit        = each.value.unit
  per_unit    = each.value.per_unit

  depends_on = [data.http.datadog_metric_metadata]
}
