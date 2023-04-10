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
    module.ExtractGrantsGovDBToXML.lambda_function_name,
  ]
}
