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
  // Since EventBridge Scheduler is not yet supported by localstack, we conditionally set the below
  // lambda_trigger local value if var.eventbridge_scheduler_enabled is false.
  eventbridge_scheduler_trigger = {
    principal  = "scheduler.amazonaws.com"
    source_arn = try(aws_scheduler_schedule.default[0].arn, "")
  }
  cloudwatch_events_trigger = {
    principal  = "events.amazonaws.com"
    source_arn = try(aws_cloudwatch_event_rule.schedule[0].arn, "")
  }
  lambda_trigger = var.eventbridge_scheduler_enabled ? local.eventbridge_scheduler_trigger : local.cloudwatch_events_trigger
  dd_tags = merge(
    {
      for item in compact(split(",", try(var.additional_environment_variables.DD_TAGS, ""))) :
      split(":", trimspace(item))[0] => try(split(":", trimspace(item))[1], "")
    },
    var.datadog_custom_tags,
    { handlername = lower(var.function_name), },
  )
}

data "aws_s3_bucket" "grants_source_data" {
  bucket = var.grants_source_data_bucket_name
}

module "lambda_execution_policy" {
  source  = "cloudposse/iam-policy/aws"
  version = "1.0.1"

  iam_source_policy_documents = var.additional_lambda_execution_policy_documents
  iam_policy_statements = {
    AllowS3Upload = {
      effect  = "Allow"
      actions = ["s3:PutObject"]
      resources = [
        # Path: /sources/YYYY/mm/dd/grants.gov/archive.zip
        "${data.aws_s3_bucket.grants_source_data.arn}/sources/*/*/*/grants.gov/archive.zip"
      ]
    }
  }
}

module "lambda_function" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "5.0.0"

  function_name = "${var.namespace}-${var.function_name}"
  description   = "Downloads and stores the daily XML database extract from Grants.gov"

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
      "task build-DownloadGrantsGovDB",
      "cd bin/DownloadGrantsGovDB",
      ":zip",
    ],
  }]
  store_on_s3               = true
  s3_bucket                 = var.lambda_artifact_bucket
  s3_server_side_encryption = "AES256"

  timeout = 120 # 2 minutes, in seconds
  environment_variables = merge(var.additional_environment_variables, {
    DD_TAGS                        = join(",", sort([for k, v in local.dd_tags : "${k}:${v}"]))
    GRANTS_GOV_BASE_URL            = "https://www.grants.gov"
    GRANTS_SOURCE_DATA_BUCKET_NAME = data.aws_s3_bucket.grants_source_data.id
    LOG_LEVEL                      = var.log_level
  })

  allowed_triggers = {
    Schedule = local.lambda_trigger
  }
}
