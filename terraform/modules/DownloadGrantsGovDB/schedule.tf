data "aws_caller_identity" "current" {}

resource "aws_iam_role" "scheduler_execution" {
  count = var.eventbridge_scheduler_enabled ? 1 : 0

  name_prefix          = "${var.namespace}-scheduler_exec"
  permissions_boundary = var.permissions_boundary_arn
  assume_role_policy   = data.aws_iam_policy_document.scheduler_execution-trust.json
}

data "aws_iam_policy_document" "scheduler_execution-trust" {
  statement {
    sid     = "AssumeRole"
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["scheduler.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [data.aws_caller_identity.current.account_id]
    }
  }
}

data "aws_iam_policy_document" "allow_invoke_lambda" {
  statement {
    sid     = "AllowInvokeLambda"
    effect  = "Allow"
    actions = ["lambda:InvokeFunction"]
    resources = [
      module.lambda_function.lambda_function_arn,
      "${module.lambda_function.lambda_function_arn}:*",
    ]
  }
}

resource "aws_iam_role_policy" "scheduler_execution-allow_invoke_lambda" {
  count = var.eventbridge_scheduler_enabled ? 1 : 0

  role   = aws_iam_role.scheduler_execution[0].id
  policy = data.aws_iam_policy_document.allow_invoke_lambda.json
}

resource "aws_scheduler_schedule" "default" {
  count = var.eventbridge_scheduler_enabled ? 1 : 0

  name                         = "${var.namespace}-${var.function_name}"
  description                  = "Invokes a Lambda function daily to download the Grants.gov DB extract"
  group_name                   = var.scheduler_group_name
  state                        = "ENABLED"
  schedule_expression          = "cron(0 5 * * ? *)"
  schedule_expression_timezone = "America/New_York"

  flexible_time_window {
    mode                      = "FLEXIBLE"
    maximum_window_in_minutes = 15
  }

  target {
    arn      = module.lambda_function.lambda_function_arn
    role_arn = aws_iam_role.scheduler_execution[0].arn
    input    = file("${path.module}/lambda_input.json")

    retry_policy {
      maximum_event_age_in_seconds = "21600" # 6 hours
    }
  }
}

resource "aws_cloudwatch_event_rule" "schedule" {
  count = var.eventbridge_scheduler_enabled ? 0 : 1

  name                = "schedule"
  description         = "Schedule for Lambda Function"
  schedule_expression = "cron(0 5 * * ? *)"
}

resource "aws_cloudwatch_event_target" "schedule_lambda" {
  count = var.eventbridge_scheduler_enabled ? 0 : 1

  rule      = aws_cloudwatch_event_rule.schedule[0].name
  target_id = module.lambda_function.lambda_function_name
  arn       = module.lambda_function.lambda_function_arn
}

resource "aws_lambda_permission" "allow_events_bridge_to_run_lambda" {
  count = var.eventbridge_scheduler_enabled ? 0 : 1

  statement_id  = "AllowExecutionFromCloudWatch"
  action        = "lambda:InvokeFunction"
  function_name = module.lambda_function.lambda_function_name
  principal     = "events.amazonaws.com"
}
