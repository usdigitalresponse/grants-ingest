resource "datadog_metric_metadata" "custom" {
  for_each = {
    for k, v in var.datadog_metrics_metadata :
    startswith(k, "grants_ingest.") ? k : "grants_ingest.${k}" => v
  }

  metric      = each.key
  short_name  = each.value.short_name
  description = each.value.description
  unit        = each.value.unit
  per_unit    = each.value.per_unit
}
