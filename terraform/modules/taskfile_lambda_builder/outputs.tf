output "s3_object_key" {
  value = aws_s3_object.lambda_function.key
}

output "local_binary_file" {
  value      = local.source_path
  depends_on = [data.external.build_command]
}

output "local_zip_file" {
  value = data.archive_file.local_zip.output_path
}
