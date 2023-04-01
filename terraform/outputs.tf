output "grants_source_data_bucket_id" {
  value = module.grants_source_data_bucket.bucket_id
}

output "grants_prepared_data_bucket_id" {
  value = module.grants_prepared_data_bucket.bucket_id
}

output "lambda_functions" {
  value = [
    module.download_grants_gov_db.lambda_function_name,
    module.split_grants_gov_db.lambda_function_name,
  ]
}
