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

data "aws_s3_bucket" "prepared_data" {
  bucket = var.grants_prepared_data_bucket_name
}

module "lambda_execution_policy" {
  source  = "cloudposse/iam-policy/aws"
  version = "0.4.0"

  iam_source_policy_documents = var.additional_lambda_execution_policy_documents
  iam_policy_statements = {
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
    AllowDynamoDBPreparedData = {
      effect = "Allow"
      actions = [
        "dynamodb:ListTables",
        "dynamodb:UpdateItem"
      ]
      resources = [var.grants_prepared_dynamodb_table_arn]
    }
  }
}

module "lambda_function" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "4.12.1"

  function_name = "${var.namespace}-${var.function_name}"
  description   = "Persists data from a prepared Grants.gov XML DB extract to DynamoDB."

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
      "task build-PersistGrantsGovXMLDB",
      "cd bin/PersistGrantsGovXMLDB",
      ":zip",
    ],
  }]
  store_on_s3               = true
  s3_bucket                 = var.lambda_artifact_bucket
  s3_server_side_encryption = "AES256"

  timeout = 30
  environment_variables = merge(var.additional_environment_variables, {
    DD_TAGS                       = join(",", sort([for k, v in local.dd_tags : "${k}:${v}"]))
    GRANTS_PREPARED_DYNAMODB_NAME = var.grants_prepared_dynamodb_table_name
    LOG_LEVEL                     = var.log_level
    S3_USE_PATH_STYLE             = "true"
  })

  allowed_triggers = {
    S3BucketNotification = {
      principal  = "s3.amazonaws.com"
      source_arn = data.aws_s3_bucket.prepared_data.arn
    }
  }
}
