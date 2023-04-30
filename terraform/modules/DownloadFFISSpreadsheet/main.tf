terraform {
  required_version = "1.3.9"
  required_providers {
    aws = "~> 4.55.0"
  }
}

locals {
  dd_tags = merge(
    {
      for item in compact(split(",", try(var.additional_environment_variables.DD_TAGS, ""))) :
      split(":", trimspace(item))[0] => try(split(":", trimspace(item))[1], "")
    },
    var.datadog_custom_tags,
    { handlername = lower(var.function_name), },
  )
}

data "aws_s3_bucket" "download_target" {
  bucket = var.download_target_bucket_name
}

data "aws_sqs_queue" "ffis_downloads" {
  name = var.source_queue_name
}

module "lambda_execution_policy" {
  source  = "cloudposse/iam-policy/aws"
  version = "0.4.0"

  iam_source_policy_documents = var.additional_lambda_execution_policy_documents
  iam_policy_statements = {
    AllowS3DownloadWrite = {
      effect  = "Allow"
      actions = ["s3:PutObject"]
      resources = [
        # Path: /sources/YYYY/mm/dd/ffis/download.xlsx
        "${data.aws_s3_bucket.download_target.arn}/sources/*/*/*/ffis/download.xlsx"
      ]
    }
    AllowSQSGet = {
      effect  = "Allow"
      actions = ["sqs:ReceiveMessage"]
      resources = [
        data.aws_sqs_queue.ffis_downloads.arn,
      ]
    }
  }
}

module "lambda_function" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "4.12.1"

  function_name = "${var.namespace}-${var.function_name}"
  description   = "Downloads FFIS XLSX files and saves to S3"

  role_permissions_boundary         = var.permissions_boundary_arn
  attach_cloudwatch_logs_policy     = true
  cloudwatch_logs_retention_in_days = var.log_retention_in_days
  attach_policy_json                = true
  policy_json                       = module.lambda_execution_policy.json

  handler       = "bootstrap"
  runtime       = "provided.al2"
  architectures = [var.lambda_arch]
  publish       = true
  layers        = var.lambda_layer_arns

  source_path = [{
    path = var.lambda_code_path
    commands = [
      "task build-DownloadFFISSpreadsheet",
      "cd bin/DownloadFFISSpreadsheet",
      ":zip",
    ],
  }]
  store_on_s3               = true
  s3_bucket                 = var.lambda_artifact_bucket
  s3_server_side_encryption = "AES256"

  timeout     = 30 # seconds
  memory_size = 128
  environment_variables = merge(var.additional_environment_variables, {
    DD_TAGS            = join(",", sort([for k, v in local.dd_tags : "${k}:${v}"]))
    FFIS_SQS_QUEUE_URL = data.aws_sqs_queue.ffis_downloads.id
    LOG_LEVEL          = var.log_level
    S3_USE_PATH_STYLE  = "true"
  })

  allowed_triggers = {
    SQSQueueNotification = {
      sqs_queue_arn = data.aws_sqs_queue.ffis_downloads.arn
      batch_size    = 1
    }
  }
}
