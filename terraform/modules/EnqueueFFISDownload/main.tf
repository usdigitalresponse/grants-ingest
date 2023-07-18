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

data "aws_s3_bucket" "source_data" {
  bucket = var.grants_source_data_bucket_name
}

data "aws_sqs_queue" "ffis_downloads" {
  name = var.destination_queue_name
}

module "lambda_execution_policy" {
  source  = "cloudposse/iam-policy/aws"
  version = "1.0.1"

  iam_source_policy_documents = var.additional_lambda_execution_policy_documents
  iam_policy_statements = {
    AllowS3DownloadSourceData = {
      effect  = "Allow"
      actions = ["s3:GetObject"]
      resources = [
        # Path: sources/YYYY/mm/dd/ffis/raw.eml
        "${data.aws_s3_bucket.source_data.arn}/sources/*/*/*/ffis/raw.eml"
      ]
    }
    AllowSQSPublish = {
      effect  = "Allow"
      actions = ["sqs:SendMessage"]
      resources = [
        data.aws_sqs_queue.ffis_downloads.arn,
      ]
    }
  }
}

module "lambda_function" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "5.3.0"

  function_name = "${var.namespace}-${var.function_name}"
  description   = "Enqueues FFIS XLSX files for download"

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
      "task build-EnqueueFFISDownload",
      "cd bin/EnqueueFFISDownload",
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
    S3BucketNotification = {
      principal  = "s3.amazonaws.com"
      source_arn = data.aws_s3_bucket.source_data.arn
    }
  }
}
