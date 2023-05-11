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

data "aws_s3_bucket" "source_data" {
  bucket = var.grants_source_data_bucket_name
}

# data "aws_dynamodb_table" "grants_data" {
#   name = var.table_name
# }

module "lambda_execution_policy" {
  source  = "cloudposse/iam-policy/aws"
  version = "0.4.0"

  iam_source_policy_documents = var.additional_lambda_execution_policy_documents
  iam_policy_statements = {
    AllowS3DownloadSourceData = {
      effect  = "Allow"
      actions = ["s3:GetObject"]
      resources = [
        # Path: sources/YYYY/mm/dd//ffis.org/v1.json
        "${data.aws_s3_bucket.source_data.arn}/sources/*/*/*/ffis.org/v1.json"
      ]
    }
    //TODO dynamo db
  }
}

module "lambda_function" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "4.12.1"

  function_name = "${var.namespace}-${var.function_name}"
  description   = "Persist FFIS data to Grants DB"

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
      "task build-PersistFFISData",
      "cd bin/PersistFFISData",
      ":zip",
    ],
  }]
  store_on_s3               = true
  s3_bucket                 = var.lambda_artifact_bucket
  s3_server_side_encryption = "AES256"

  timeout     = 30 # seconds
  memory_size = 128
  environment_variables = merge(var.additional_environment_variables, {
    DD_TAGS           = join(",", sort([for k, v in local.dd_tags : "${k}:${v}"]))
    LOG_LEVEL         = var.log_level
    S3_USE_PATH_STYLE = "true"
  })

  allowed_triggers = {
    S3BucketNotification = {
      principal  = "s3.amazonaws.com"
      source_arn = data.aws_s3_bucket.source_data.arn
    }
  }
}
