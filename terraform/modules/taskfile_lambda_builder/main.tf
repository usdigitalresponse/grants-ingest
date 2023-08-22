locals {
  default_source_path = join("/", [
    trimsuffix(var.binary_base_path, "/"),
    var.function_name,
    "bootstrap"
  ])
  source_path = coalesce(var.override_path_to_binary, local.default_source_path)

  default_output_path = join("/", [trimsuffix(path.root, "/"), "builds"])
  output_path = join("/", [
    trimsuffix(coalesce(var.override_artifact_base_path, local.default_output_path), "/"),
    "${var.function_name}.zip"
  ])

  default_task_command = "build-${var.function_name}"
  task_command         = coalesce(var.override_taskfile_command, local.default_task_command)
}

data "external" "build_command" {
  program = ["${path.module}/script.bash"]
  query   = { task_command = local.task_command }
}

data "archive_file" "local_zip" {
  type             = "zip"
  source_file      = local.source_path
  output_path      = local.output_path
  output_file_mode = "0644"

  depends_on = [data.external.build_command]
}

resource "aws_s3_object" "lambda_function" {
  bucket                 = var.s3_bucket
  key                    = "${var.s3_key_prefix}${data.archive_file.local_zip.output_md5}.zip"
  source                 = data.archive_file.local_zip.output_path
  server_side_encryption = "AES256"

  depends_on = [data.archive_file.local_zip]
}
