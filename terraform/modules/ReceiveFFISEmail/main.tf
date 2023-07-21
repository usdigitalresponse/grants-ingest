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

  allowed_email_senders = join(",", [
    for v in sort(var.allowed_email_senders) : lower(trimspace(v))
  ])
}

data "aws_s3_bucket" "email_delivery" {
  bucket = var.email_delivery_bucket_name
}

data "aws_s3_bucket" "grants_source_data" {
  name = var.grants_source_data_bucket_name
}

module "lambda_execution_policy" {
  source  = "cloudposse/iam-policy/aws"
  version = "1.0.1"

  iam_source_policy_documents = var.additional_lambda_execution_policy_documents
  iam_policy_statements = {
    AllowS3DownloadNewEmails = {
      effect  = "Allow"
      actions = ["s3:GetObject", "s3:GetObjectTagging"]
      resources = [
        "${data.aws_s3_bucket.email_delivery.arn}/${var.email_delivery_object_key_prefix}/*"
      ]
    }
    AllowS3UploadVerifiedEmails = {
      effect  = "Allow"
      actions = ["s3:PutObject", "s3:PutObjectTagging"]
      resources = [
        # Path: sources/YYYY/mm/dd/ffis/raw.eml
        "${data.aws_s3_bucket.grants_source_data.arn}/sources/*/*/*/ffis/raw.eml"
      ]
    }
  }
}

module "lambda_function" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "5.3.0"

  function_name = "${var.namespace}-${var.function_name}"
  description   = "Receives and verifies new FFIS digest emails"

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
      "task build-ReceiveFFISEmail",
      "cd bin/ReceiveFFISEmail",
      ":zip",
    ],
  }]
  store_on_s3               = true
  s3_bucket                 = var.lambda_artifact_bucket
  s3_server_side_encryption = "AES256"

  timeout     = 30 # seconds
  memory_size = 128
  environment_variables = merge(var.additional_environment_variables, {
    DD_TAGS                        = join(",", sort([for k, v in local.dd_tags : "${k}:${v}"]))
    LOG_LEVEL                      = var.log_level
    ALLOWED_EMAIL_SENDERS          = local.allowed_email_senders
    GRANTS_SOURCE_DATA_BUCKET_NAME = data.aws_s3_bucket.grants_source_data.id
  })

  allowed_triggers = {
    S3BucketNotification = {
      principal  = "s3.amazonaws.com"
      source_arn = data.aws_s3_bucket.email_delivery.arn
    }
  }
}
