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

resource "aws_cloudwatch_event_bus" "target" {
  name = "${var.namespace}-grant_events"
}

resource "aws_sqs_queue" "dlq" {
  name = "publish_grant_events_dlq"

  visibility_timeout_seconds = 3600 // 1 hour
  delay_seconds              = 0
  receive_wait_time_seconds  = 20
  message_retention_seconds  = 1209600 // 14 days
  max_message_size           = 262144  // 256 kB
  sqs_managed_sse_enabled    = true
}

data "aws_dynamodb_table" "source" {
  name = var.dynamodb_table_name
}

module "lambda_function" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "5.2.0"

  function_name = "${var.namespace}-${var.function_name}"
  description   = "Publishes grant opportunity create/update events from DynamoDB to EventBridge."

  role_permissions_boundary         = var.permissions_boundary_arn
  attach_cloudwatch_logs_policy     = true
  cloudwatch_logs_retention_in_days = var.log_retention_in_days
  attach_policy_jsons               = true
  number_of_policy_jsons            = length(var.additional_lambda_execution_policy_documents)
  policy_jsons                      = var.additional_lambda_execution_policy_documents
  attach_policy_statements          = true
  policy_statements = {
    PublishToEventBridge = {
      effect    = "Allow"
      actions   = ["events:PutEvents"]
      resources = [aws_cloudwatch_event_bus.target.arn]
    }
    PublishFailuresToDLQ = {
      effect    = "Allow"
      actions   = ["sqs:SendMessage"]
      resources = [aws_sqs_queue.dlq.arn]
    }
    StreamRecordsFromDynamoDB = {
      effect = "Allow"
      actions = [
        "dynamodb:DescribeStream",
        "dynamodb:GetRecords",
        "dynamodb:GetShardIterator",
      ]
      resources = [data.aws_dynamodb_table.source.stream_arn]
    }
    ListDynamoDBStreams = {
      effect    = "Allow"
      actions   = ["dynamodb:ListStreams"]
      resources = ["*"]
    }
  }

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
    DD_TAGS        = join(",", sort([for k, v in local.dd_tags : "${k}:${v}"]))
    LOG_LEVEL      = var.log_level
    EVENT_BUS_NAME = aws_cloudwatch_event_bus.target.name
  })

  event_source_mapping = {
    dynamodb = {
      event_source_arn           = data.aws_dynamodb_table.source.stream_arn
      starting_position          = "LATEST"
      parallelization_factor     = 10
      function_response_types    = ["ReportBatchItemFailures"]
      bisect_batch_on_error      = true
      destination_arn_on_failure = aws_sqs_queue.dlq.arn
    }
  }

  allowed_triggers = {
    dynamodb = {
      principal  = "dynamodb.amazonaws.com"
      source_arn = data.aws_dynamodb_table.source.stream_arn
    }
  }
}
