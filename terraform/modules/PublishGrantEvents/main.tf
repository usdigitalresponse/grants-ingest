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

data "aws_sqs_queue" "grants_dlq" {
  name = var.dlq_arn
}

module "lambda_execution_policy" {
  source  = "cloudposse/iam-policy/aws"
  version = "0.4.0"

  iam_source_policy_documents = var.additional_lambda_execution_policy_documents
  iam_policy_statements = {
    AllowDLQPublish = {
      effect  = "Allow"
      actions = ["sqs.SendMessage"]
      resources  = [var.dlq_arn]
    }
    AllowEventBusPublish = {
      effect  = "Allow"
      actions = ["events.PutEvents"]
      resources = [var.grants_event_bus_arn]
    }
  }
}

module "lambda_function" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "4.12.1"

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
      "task build-PublishGrantEvents",
      "cd bin/PublishGrantEvents",
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
    DLQ_SQS_QUEUE_URL  = data.aws_sqs_queue.grants_dlq.id
    LOG_LEVEL          = var.log_level
    S3_USE_PATH_STYLE  = "true"
  })
}