terraform {
  required_version = "1.5.1"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.4.0"
    }
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
  version = "2.0.0"

  iam_source_policy_documents = var.additional_lambda_execution_policy_documents
  iam_policy_statements = {
    AllowS3DownloadWrite = {
      effect  = "Allow"
      actions = ["s3:PutObject"]
      resources = [
        # Path: /sources/YYYY/mm/dd/ffis.org/download.xlsx
        "${data.aws_s3_bucket.download_target.arn}/sources/*/*/*/ffis.org/download.xlsx"
      ]
    }
    AllowSQSGet = {
      effect  = "Allow"
      actions = ["sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes"]
      resources = [
        data.aws_sqs_queue.ffis_downloads.arn,
      ]
    }
  }
}

module "lambda_artifact" {
  source = "../taskfile_lambda_builder"

  binary_base_path = var.lambda_binaries_base_path
  function_name    = var.function_name
  s3_bucket        = var.lambda_artifact_bucket
}

module "lambda_function" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "5.3.0"

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

  create_package = false
  s3_existing_package = {
    bucket = var.lambda_artifact_bucket
    key    = module.lambda_artifact.s3_object_key
  }

  timeout     = 30 # seconds
  memory_size = 128
  environment_variables = merge(var.additional_environment_variables, {
    DD_TAGS            = join(",", sort([for k, v in local.dd_tags : "${k}:${v}"]))
    TARGET_BUCKET_NAME = data.aws_s3_bucket.download_target.id
    LOG_LEVEL          = var.log_level
  })

  event_source_mapping = {
    sqs = {
      enabled                            = true
      batch_size                         = 1
      maximum_batching_window_in_seconds = 20
      event_source_arn                   = data.aws_sqs_queue.ffis_downloads.arn
      scaling_config = {
        maximum_concurrency = 2
      }
    }
  }

  allowed_triggers = {
    SQSQueueNotification = {
      principal     = "sqs.amazonaws.com"
      sqs_queue_arn = data.aws_sqs_queue.ffis_downloads.arn
      batch_size    = 1
    }
  }
}
