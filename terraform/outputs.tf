output "grants_source_data_bucket_id" {
  value = module.grants_source_data_bucket.bucket_id
}

output "grants_prepared_data_bucket_id" {
  value = module.grants_prepared_data_bucket.bucket_id
}

output "lambda_functions" {
  value = [
    module.DownloadGrantsGovDB.lambda_function_name,
    module.SplitGrantsGovXMLDB.lambda_function_name,
    module.EnqueueFFISDownload.lambda_function_name,
    module.DownloadFFISSpreadsheet.lambda_function_name,
    module.PersistFFISData.lambda_function_name,
  ]
}
