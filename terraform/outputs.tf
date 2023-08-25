output "grants_source_data_bucket_id" {
  value = module.grants_source_data_bucket.bucket_id
}

output "grants_prepared_data_bucket_id" {
  value = module.grants_prepared_data_bucket.bucket_id
}

output "service_dashboard_url" {
  value = var.datadog_dashboards_enabled ? "https://app.datadoghq.com${one(datadog_dashboard.service_dashboard[*].url)}" : null
}

output "lambda_functions" {
  value = [
    module.DownloadGrantsGovDB.lambda_function_name,
    module.ReceiveFFISEmail.lambda_function_name,
    module.EnqueueFFISDownload.lambda_function_name,
    module.DownloadFFISSpreadsheet.lambda_function_name,
    module.SplitGrantsGovXMLDB.lambda_function_name,
    module.SplitFFISSpreadsheet.lambda_function_name,
    module.ExtractGrantsGovDBToXML.lambda_function_name,
    module.PersistGrantsGovXMLDB.lambda_function_name,
    module.PersistFFISData.lambda_function_name,
    module.PublishGrantEvents.lambda_function_name,
  ]
}
