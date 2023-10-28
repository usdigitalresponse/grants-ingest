terraform {
  required_version = "1.5.1"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.22.0"
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

data "aws_s3_bucket" "prepared_data" {
  bucket = var.grants_prepared_data_bucket_name
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
        # Path: sources/YYYY/mm/dd/grants.gov/extract.xml
        "${data.aws_s3_bucket.source_data.arn}/sources/*/*/*/grants.gov/extract.xml"
      ]
    }
    AllowInspectS3PreparedData = {
      effect = "Allow"
      actions = [
        "s3:GetObject",
        "s3:ListBucket"
      ]
      resources = [
        data.aws_s3_bucket.prepared_data.arn,
        "${data.aws_s3_bucket.prepared_data.arn}/*/*/grants.gov/v2.xml"
      ]
    }
    AllowS3UploadPreparedData = {
      effect  = "Allow"
      actions = ["s3:PutObject"]
      resources = [
        # Path: <first 3 digits of grant ID>/<grant id>/grants.gov/v2.xml
        "${data.aws_s3_bucket.prepared_data.arn}/*/*/grants.gov/v2.xml"
      ]
    }
  }
}

module "lambda_artifact" {
  source = "../taskfile_lambda_builder"

  autobuild        = var.lambda_autobuild
  binary_base_path = var.lambda_binaries_base_path
  function_name    = var.function_name
  s3_bucket        = var.lambda_artifact_bucket
}

module "lambda_function" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "6.2.0"

  function_name = "${var.namespace}-${var.function_name}"
  description   = "Creates per-grant XML data files from a source Grants.gov XML DB extract."

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

  timeout     = 300 # 5 minutes, in seconds
  memory_size = 1024
  environment_variables = merge(var.additional_environment_variables, {
    DD_TRACE_RATE_LIMIT              = "1000"
    DD_TAGS                          = join(",", sort([for k, v in local.dd_tags : "${k}:${v}"]))
    DOWNLOAD_CHUNK_LIMIT             = "20"
    GRANTS_PREPARED_DATA_BUCKET_NAME = data.aws_s3_bucket.prepared_data.id
    LOG_LEVEL                        = var.log_level
    MAX_CONCURRENT_UPLOADS           = "10"
  })

  allowed_triggers = {
    S3BucketNotification = {
      principal  = "s3.amazonaws.com"
      source_arn = data.aws_s3_bucket.source_data.arn
    }
  }
}
