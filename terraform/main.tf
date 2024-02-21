terraform {
  required_version = "1.5.1"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.37.0"
    }
    datadog = {
      source  = "DataDog/datadog"
      version = "~> 3.35.0"
    }
    http = {
      source  = "hashicorp/http"
      version = "3.4.1"
    }
  }
  backend "s3" {}
}

provider "aws" {
  default_tags {
    tags = merge(
      {
        env        = var.environment
        management = "terraform"
        owner      = "grants"
        repo       = "grants-ingest"
        service    = "grants-ingest"
        usage      = "workload"
      },
      var.tags
    )
  }
}

provider "datadog" {
  validate = var.datadog_api_key != "" && var.datadog_app_key != "" ? true : false
  api_key  = var.datadog_api_key
  app_key  = var.datadog_app_key
}

data "aws_region" "current" {}
data "aws_partition" "current" {}
data "aws_caller_identity" "current" {}

locals {
  lambda_binaries_base_path = coalesce(var.lambda_binaries_base_path, "${path.module}/../bin")
  permissions_boundary_arn = !can(coalesce(var.permissions_boundary_policy_name)) ? null : join(":", [
    "arn",
    data.aws_partition.current.id,
    "iam",
    "",
    data.aws_caller_identity.current.account_id,
    "policy/${var.permissions_boundary_policy_name}"
  ])

  datadog_extension_layer_arn = join(":", [
    "arn",
    data.aws_partition.current.id,
    "lambda",
    data.aws_region.current.name,
    { aws = "464622532012", aws-us-gov = "002406178527" }[data.aws_partition.current.id],
    "layer",
    format("Datadog-Extension%s", var.lambda_arch == "arm64" ? "-ARM" : ""),
    var.datadog_lambda_extension_version,
  ])

  source_data_bucket_temp_storage_path_prefix = "tmp"
}

module "this" {
  source  = "cloudposse/label/null"
  version = "0.25.0"

  enabled   = var.enabled
  namespace = var.namespace
  tags      = var.tags
}

module "s3_label" {
  source  = "cloudposse/label/null"
  version = "0.25.0"

  context = module.this.context
  attributes = [
    data.aws_caller_identity.current.account_id,
    data.aws_region.current.name,
  ]
}

module "lambda_artifacts_bucket" {
  source  = "cloudposse/s3-bucket/aws"
  version = "4.0.1"
  context = module.s3_label.context
  name    = "lambda_artifacts"

  acl                          = "private"
  versioning_enabled           = true
  sse_algorithm                = "AES256"
  allow_ssl_requests_only      = true
  allow_encrypted_uploads_only = true
  source_policy_documents      = []

  lifecycle_configuration_rules = [
    {
      enabled                                = true
      id                                     = "rule-1"
      filter_and                             = null
      abort_incomplete_multipart_upload_days = 7
      transition                             = [{ days = null }]
      expiration                             = { days = null }
      noncurrent_version_transition = [
        {
          noncurrent_days = 30
          storage_class   = "GLACIER"
        },
      ]
      noncurrent_version_expiration = {
        noncurrent_days = 90
      }
    }
  ]
}

module "grants_source_data_bucket" {
  source  = "cloudposse/s3-bucket/aws"
  version = "4.0.1"
  context = module.s3_label.context
  name    = "grants_source_data"

  acl                          = "private"
  versioning_enabled           = true
  sse_algorithm                = "AES256"
  allow_ssl_requests_only      = true
  allow_encrypted_uploads_only = true
  source_policy_documents      = []

  lifecycle_configuration_rules = [
    {
      enabled = true
      id      = "rule-1"
      filter_and = {
        object_size_greater_than = null
        object_size_less_than    = null
        prefix                   = null
        tags                     = {}
      }
      abort_incomplete_multipart_upload_days = 1
      transition                             = [{ days = null }]
      expiration                             = { days = null }
      noncurrent_version_transition = [
        {
          noncurrent_days = 30
          storage_class   = "GLACIER"
        },
      ]
      noncurrent_version_expiration = {
        noncurrent_days = 2557 # 7 years (includes 2 leap days)
      }
    },
    {
      // Ensures "tmp/"-prefixed objects are deleted after 7 days
      enabled = true
      id      = "temporary-storage-cleanup"
      filter_and = {
        object_size_greater_than = null
        object_size_less_than    = null
        prefix                   = "${local.source_data_bucket_temp_storage_path_prefix}/"
        tags                     = {}
      }
      abort_incomplete_multipart_upload_days = 1
      transition                             = [{ days = null }]
      expiration                             = { days = 7 }
      noncurrent_version_transition          = [{ noncurrent_days = null }]
      noncurrent_version_expiration          = { noncurrent_days = null }
    },
  ]
}

module "grants_prepared_data_bucket" {
  source  = "cloudposse/s3-bucket/aws"
  version = "4.0.1"
  context = module.s3_label.context
  name    = "grants_prepared_data"

  acl                          = "private"
  versioning_enabled           = true
  sse_algorithm                = "AES256"
  allow_ssl_requests_only      = true
  allow_encrypted_uploads_only = true
  source_policy_documents      = []

  lifecycle_configuration_rules = [
    {
      enabled                                = true
      id                                     = "rule-1"
      filter_and                             = null
      abort_incomplete_multipart_upload_days = 1
      transition                             = [{ days = null }]
      expiration                             = { days = null }
      noncurrent_version_transition = [
        {
          noncurrent_days = 30
          storage_class   = "GLACIER"
        },
      ]
      noncurrent_version_expiration = {
        noncurrent_days = 2557 # 7 years (includes 2 leap days)
      }
    }
  ]
}

module "email_delivery_bucket" {
  source  = "cloudposse/s3-bucket/aws"
  version = "4.0.1"
  context = module.s3_label.context
  name    = "email_delivery"

  acl                          = "private"
  versioning_enabled           = true
  sse_algorithm                = "AES256"
  allow_ssl_requests_only      = true
  allow_encrypted_uploads_only = false
  source_policy_documents      = [data.aws_iam_policy_document.ses_source_data_s3_access.json]

  lifecycle_configuration_rules = [
    {
      enabled                                = true
      id                                     = "rule-1"
      filter_and                             = null
      abort_incomplete_multipart_upload_days = 1
      transition                             = [{ days = null }]
      expiration                             = { days = 30 }
      noncurrent_version_transition          = [{ noncurrent_days = null }]
      noncurrent_version_expiration          = { noncurrent_days = null }
    }
  ]
}

resource "aws_scheduler_schedule_group" "default" {
  count = var.eventbridge_scheduler_enabled ? 1 : 0

  name = var.namespace
}

data "aws_ssm_parameter" "datadog_api_key_secret_arn" {
  count = var.datadog_enabled ? 1 : 0

  name = "${var.ssm_deployment_parameters_path_prefix}/datadog/api_key_secret_arn"
}

data "aws_iam_policy_document" "read_datadog_api_key_secret" {
  count = var.datadog_enabled ? 1 : 0

  statement {
    sid       = "GetDatadogAPIKeySecretValue"
    effect    = "Allow"
    actions   = ["secretsmanager:GetSecretValue"]
    resources = [data.aws_ssm_parameter.datadog_api_key_secret_arn[0].value]
  }
}

module "grants_prepared_dynamodb_table" {
  source  = "cloudposse/dynamodb/aws"
  version = "0.35.0"
  context = module.this.context

  name                          = "prepareddata"
  hash_key                      = "grant_id"
  table_class                   = "STANDARD"
  billing_mode                  = "PAY_PER_REQUEST"
  enable_streams                = true
  stream_view_type              = "NEW_AND_OLD_IMAGES"
  enable_point_in_time_recovery = true
  enable_encryption             = true
}

resource "aws_dynamodb_contributor_insights" "grants_prepared_dynamodb_main" {
  count = var.dynamodb_contributor_insights_enabled ? 1 : 0

  table_name = module.grants_prepared_dynamodb_table.table_name
}

resource "aws_ses_receipt_rule_set" "ffis_ingest" {
  rule_set_name = "${var.namespace}-ffis_ingest"
}

resource "aws_ses_active_receipt_rule_set" "active" {
  count = var.ses_active_receipt_rule_set_enabled ? 1 : 0

  rule_set_name = aws_ses_receipt_rule_set.ffis_ingest.rule_set_name
}

resource "aws_ses_receipt_rule" "ffis_ingest" {
  name          = "${var.namespace}-ffis_ingest"
  rule_set_name = aws_ses_receipt_rule_set.ffis_ingest.rule_set_name
  recipients    = [var.ffis_ingest_email_address]
  enabled       = true
  scan_enabled  = true
  tls_policy    = "Require"

  s3_action {
    position          = 1
    bucket_name       = module.email_delivery_bucket.bucket_id
    object_key_prefix = "ses/ffis_ingest/new/"
  }

  depends_on = [
    module.email_delivery_bucket,
  ]
}

resource "aws_sqs_queue" "ffis_downloads" {
  name = "${var.namespace}-ffis_downloads"

  delay_seconds              = 0
  visibility_timeout_seconds = 15 * 60
  receive_wait_time_seconds  = 20
  message_retention_seconds  = 5 * 60 * 60 * 24 # 5 days
  max_message_size           = 1024             # 1 KB
  sqs_managed_sse_enabled    = true

  lifecycle {
    prevent_destroy = true
  }
}

data "aws_iam_policy_document" "ses_source_data_s3_access" {
  statement {
    sid = "AllowFFISEmailDeliveryFromSES"
    principals {
      type        = "Service"
      identifiers = ["ses.amazonaws.com"]
    }

    actions = [
      "s3:PutObject",
    ]

    resources = [
      "arn:aws:s3:::${module.s3_label.namespace}-emaildelivery-${data.aws_caller_identity.current.account_id}-${data.aws_region.current.name}/ses/*",
    ]

    condition {
      test     = "StringEquals"
      variable = "AWS:SourceArn"
      values = [
        "arn:aws:ses:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:receipt-rule-set/${var.namespace}-ffis_ingest:receipt-rule/${var.namespace}-ffis_ingest"
      ]
    }
    condition {
      test     = "StringEquals"
      variable = "AWS:SourceAccount"
      values = [
        data.aws_caller_identity.current.account_id
      ]
    }
  }
}

// Lambda defaults
locals {
  datadog_custom_tags = merge(
    { "git.repository_url" = var.git_repository_url, "git.commit.sha" = var.git_commit_sha },
    var.datadog_lambda_custom_tags,
  )
  lambda_environment_variables = merge(
    !var.datadog_enabled ? {} : merge(
      {
        DD_API_KEY_SECRET_ARN        = data.aws_ssm_parameter.datadog_api_key_secret_arn[0].value
        DD_APM_ENABLED               = "true"
        DD_CAPTURE_LAMBDA_PAYLOAD    = "true"
        DD_ENV                       = var.environment
        DD_SERVERLESS_APPSEC_ENABLED = "true"
        DD_SERVICE                   = "grants-ingest"
        DD_SITE                      = "datadoghq.com"
        DD_TAGS                      = join(",", sort([for k, v in local.datadog_custom_tags : "${k}:${v}"]))
        DD_TRACE_ENABLED             = "true"
        DD_VERSION                   = var.version_identifier
      },
      var.datadog_reserved_tags, // Allow conflicting variable-defined tags to override the above defaults
    ),
    {
      TZ = "UTC"
    },
    // Allow conflicting variable-defined environment variables the override of the above
    var.additional_lambda_environment_variables,
  )
  lambda_execution_policies = compact([
    try(data.aws_iam_policy_document.read_datadog_api_key_secret[0].json, ""),
  ])
  lambda_layer_arns = compact([
    var.datadog_enabled ? local.datadog_extension_layer_arn : "",
  ])
}

resource "aws_s3_bucket_notification" "grant_source_data" {
  bucket = module.grants_source_data_bucket.bucket_id

  lambda_function {
    lambda_function_arn = module.ExtractGrantsGovDBToXML.lambda_function_arn
    events              = ["s3:ObjectCreated:*"]
    filter_prefix       = "sources/"
    filter_suffix       = "/grants.gov/archive.zip"
  }

  lambda_function {
    lambda_function_arn = module.SplitGrantsGovXMLDB.lambda_function_arn
    events              = ["s3:ObjectCreated:*"]
    filter_prefix       = "sources/"
    filter_suffix       = "/grants.gov/extract.xml"
  }

  lambda_function {
    lambda_function_arn = module.EnqueueFFISDownload.lambda_function_arn
    events              = ["s3:ObjectCreated:*"]
    filter_prefix       = "sources/"
    filter_suffix       = "/ffis.org/raw.eml"
  }

  lambda_function {
    lambda_function_arn = module.SplitFFISSpreadsheet.lambda_function_arn
    events              = ["s3:ObjectCreated:*"]
    filter_prefix       = "sources/"
    filter_suffix       = "/ffis.org/download.xlsx"
  }

  depends_on = [
    module.grants_source_data_bucket,
    module.ExtractGrantsGovDBToXML,
    module.SplitGrantsGovXMLDB,
    module.EnqueueFFISDownload,
    module.SplitFFISSpreadsheet,
  ]
}

resource "aws_s3_bucket_notification" "grant_prepared_data" {
  bucket = module.grants_prepared_data_bucket.bucket_id

  lambda_function {
    lambda_function_arn = module.PersistGrantsGovXMLDB.lambda_function_arn
    events              = ["s3:ObjectCreated:*"]
    filter_suffix       = "/grants.gov/v2.xml"
  }

  lambda_function {
    lambda_function_arn = module.PersistFFISData.lambda_function_arn
    events              = ["s3:ObjectCreated:*"]
    filter_suffix       = "/ffis.org/v1.json"
  }

  depends_on = [
    module.grants_prepared_data_bucket,
    module.PersistGrantsGovXMLDB,
    module.PersistFFISData,
  ]
}

resource "aws_s3_bucket_notification" "email_delivery" {
  bucket = module.email_delivery_bucket.bucket_id

  lambda_function {
    lambda_function_arn = module.ReceiveFFISEmail.lambda_function_arn
    events              = ["s3:ObjectCreated:*"]
    filter_prefix       = one(aws_ses_receipt_rule.ffis_ingest.s3_action).object_key_prefix
  }

  depends_on = [
    module.email_delivery_bucket,
    module.ReceiveFFISEmail,
  ]
}

// Modules providing Lambda functions
module "DownloadGrantsGovDB" {
  source = "./modules/DownloadGrantsGovDB"

  namespace                                    = var.namespace
  function_name                                = "DownloadGrantsGovDB"
  permissions_boundary_arn                     = local.permissions_boundary_arn
  lambda_artifact_bucket                       = module.lambda_artifacts_bucket.bucket_id
  log_retention_in_days                        = var.lambda_default_log_retention_in_days
  log_level                                    = var.lambda_default_log_level
  lambda_autobuild                             = var.lambda_binaries_autobuild
  lambda_binaries_base_path                    = local.lambda_binaries_base_path
  lambda_arch                                  = var.lambda_arch
  additional_environment_variables             = local.lambda_environment_variables
  additional_lambda_execution_policy_documents = local.lambda_execution_policies
  lambda_layer_arns                            = local.lambda_layer_arns

  scheduler_group_name           = try(aws_scheduler_schedule_group.default[0].name, "")
  grants_source_data_bucket_name = module.grants_source_data_bucket.bucket_id
  eventbridge_scheduler_enabled  = var.eventbridge_scheduler_enabled
}

module "SplitGrantsGovXMLDB" {
  source = "./modules/SplitGrantsGovXMLDB"

  namespace                                    = var.namespace
  function_name                                = "SplitGrantsGovXMLDB"
  permissions_boundary_arn                     = local.permissions_boundary_arn
  lambda_artifact_bucket                       = module.lambda_artifacts_bucket.bucket_id
  log_retention_in_days                        = var.lambda_default_log_retention_in_days
  log_level                                    = var.lambda_default_log_level
  lambda_autobuild                             = var.lambda_binaries_autobuild
  lambda_binaries_base_path                    = local.lambda_binaries_base_path
  lambda_arch                                  = var.lambda_arch
  additional_environment_variables             = local.lambda_environment_variables
  additional_lambda_execution_policy_documents = local.lambda_execution_policies
  lambda_layer_arns                            = local.lambda_layer_arns

  grants_source_data_bucket_name   = module.grants_source_data_bucket.bucket_id
  grants_prepared_data_bucket_name = module.grants_prepared_data_bucket.bucket_id
}

module "ReceiveFFISEmail" {
  source = "./modules/ReceiveFFISEmail"

  namespace                                    = var.namespace
  function_name                                = "ReceiveFFISEmail"
  permissions_boundary_arn                     = local.permissions_boundary_arn
  lambda_artifact_bucket                       = module.lambda_artifacts_bucket.bucket_id
  log_retention_in_days                        = var.lambda_default_log_retention_in_days
  log_level                                    = var.lambda_default_log_level
  lambda_autobuild                             = var.lambda_binaries_autobuild
  lambda_binaries_base_path                    = local.lambda_binaries_base_path
  lambda_arch                                  = var.lambda_arch
  additional_environment_variables             = local.lambda_environment_variables
  additional_lambda_execution_policy_documents = local.lambda_execution_policies
  lambda_layer_arns                            = local.lambda_layer_arns

  email_delivery_bucket_name       = one(aws_ses_receipt_rule.ffis_ingest.s3_action).bucket_name
  email_delivery_object_key_prefix = one(aws_ses_receipt_rule.ffis_ingest.s3_action).object_key_prefix
  grants_source_data_bucket_name   = module.grants_source_data_bucket.bucket_id
  allowed_email_senders            = var.ffis_email_allowed_senders

  depends_on = [
    module.email_delivery_bucket,
    aws_ses_receipt_rule.ffis_ingest,
    module.grants_source_data_bucket,
  ]
}

module "EnqueueFFISDownload" {
  source = "./modules/EnqueueFFISDownload"

  namespace                                    = var.namespace
  function_name                                = "EnqueueFFISDownload"
  permissions_boundary_arn                     = local.permissions_boundary_arn
  lambda_artifact_bucket                       = module.lambda_artifacts_bucket.bucket_id
  log_retention_in_days                        = var.lambda_default_log_retention_in_days
  log_level                                    = var.lambda_default_log_level
  lambda_autobuild                             = var.lambda_binaries_autobuild
  lambda_binaries_base_path                    = local.lambda_binaries_base_path
  lambda_arch                                  = var.lambda_arch
  additional_environment_variables             = local.lambda_environment_variables
  additional_lambda_execution_policy_documents = local.lambda_execution_policies
  lambda_layer_arns                            = local.lambda_layer_arns
  destination_queue_name                       = aws_sqs_queue.ffis_downloads.name

  grants_source_data_bucket_name = module.grants_source_data_bucket.bucket_id

  depends_on = [
    module.grants_source_data_bucket,
    aws_sqs_queue.ffis_downloads,
  ]
}

module "PersistGrantsGovXMLDB" {
  source = "./modules/PersistGrantsGovXMLDB"

  namespace                                    = var.namespace
  function_name                                = "PersistGrantsGovXMLDB"
  permissions_boundary_arn                     = local.permissions_boundary_arn
  lambda_artifact_bucket                       = module.lambda_artifacts_bucket.bucket_id
  log_retention_in_days                        = var.lambda_default_log_retention_in_days
  log_level                                    = var.lambda_default_log_level
  lambda_autobuild                             = var.lambda_binaries_autobuild
  lambda_binaries_base_path                    = local.lambda_binaries_base_path
  lambda_arch                                  = var.lambda_arch
  additional_environment_variables             = local.lambda_environment_variables
  additional_lambda_execution_policy_documents = local.lambda_execution_policies
  lambda_layer_arns                            = local.lambda_layer_arns

  grants_prepared_data_bucket_name    = module.grants_prepared_data_bucket.bucket_id
  grants_prepared_dynamodb_table_name = module.grants_prepared_dynamodb_table.table_name
  grants_prepared_dynamodb_table_arn  = module.grants_prepared_dynamodb_table.table_arn
}

module "DownloadFFISSpreadsheet" {
  source = "./modules/DownloadFFISSpreadsheet"

  namespace                                    = var.namespace
  function_name                                = "DownloadFFISSpreadsheet"
  permissions_boundary_arn                     = local.permissions_boundary_arn
  lambda_artifact_bucket                       = module.lambda_artifacts_bucket.bucket_id
  log_retention_in_days                        = var.lambda_default_log_retention_in_days
  log_level                                    = var.lambda_default_log_level
  lambda_autobuild                             = var.lambda_binaries_autobuild
  lambda_binaries_base_path                    = local.lambda_binaries_base_path
  lambda_arch                                  = var.lambda_arch
  additional_environment_variables             = local.lambda_environment_variables
  additional_lambda_execution_policy_documents = local.lambda_execution_policies
  lambda_layer_arns                            = local.lambda_layer_arns

  source_queue_name           = aws_sqs_queue.ffis_downloads.name
  download_target_bucket_name = module.grants_source_data_bucket.bucket_id

  depends_on = [
    module.grants_source_data_bucket,
    aws_sqs_queue.ffis_downloads,
  ]
}

module "SplitFFISSpreadsheet" {
  source = "./modules/SplitFFISSpreadsheet"

  namespace                                    = var.namespace
  function_name                                = "SplitFFISSpreadsheet"
  permissions_boundary_arn                     = local.permissions_boundary_arn
  lambda_artifact_bucket                       = module.lambda_artifacts_bucket.bucket_id
  log_retention_in_days                        = var.lambda_default_log_retention_in_days
  log_level                                    = var.lambda_default_log_level
  lambda_autobuild                             = var.lambda_binaries_autobuild
  lambda_binaries_base_path                    = local.lambda_binaries_base_path
  lambda_arch                                  = var.lambda_arch
  additional_environment_variables             = local.lambda_environment_variables
  additional_lambda_execution_policy_documents = local.lambda_execution_policies
  lambda_layer_arns                            = local.lambda_layer_arns

  grants_source_data_bucket_name   = module.grants_source_data_bucket.bucket_id
  grants_prepared_data_bucket_name = module.grants_prepared_data_bucket.bucket_id

  depends_on = [
    module.grants_source_data_bucket,
    module.grants_prepared_data_bucket,
    aws_sqs_queue.ffis_downloads,
  ]
}

module "PersistFFISData" {
  source = "./modules/PersistFFISData"

  namespace                                    = var.namespace
  function_name                                = "PersistFFISData"
  permissions_boundary_arn                     = local.permissions_boundary_arn
  lambda_artifact_bucket                       = module.lambda_artifacts_bucket.bucket_id
  log_retention_in_days                        = var.lambda_default_log_retention_in_days
  log_level                                    = var.lambda_default_log_level
  lambda_autobuild                             = var.lambda_binaries_autobuild
  lambda_binaries_base_path                    = local.lambda_binaries_base_path
  lambda_arch                                  = var.lambda_arch
  additional_environment_variables             = local.lambda_environment_variables
  additional_lambda_execution_policy_documents = local.lambda_execution_policies
  lambda_layer_arns                            = local.lambda_layer_arns

  grants_prepared_data_bucket_name    = module.grants_prepared_data_bucket.bucket_id
  grants_prepared_dynamodb_table_name = module.grants_prepared_dynamodb_table.table_name
  grants_prepared_dynamodb_table_arn  = module.grants_prepared_dynamodb_table.table_arn

  depends_on = [
    module.grants_source_data_bucket,
  ]
}

module "ExtractGrantsGovDBToXML" {
  source = "./modules/ExtractGrantsGovDBToXML"

  namespace                                    = var.namespace
  function_name                                = "ExtractGrantsGovDBToXML"
  permissions_boundary_arn                     = local.permissions_boundary_arn
  lambda_artifact_bucket                       = module.lambda_artifacts_bucket.bucket_id
  log_retention_in_days                        = var.lambda_default_log_retention_in_days
  log_level                                    = var.lambda_default_log_level
  lambda_autobuild                             = var.lambda_binaries_autobuild
  lambda_binaries_base_path                    = local.lambda_binaries_base_path
  lambda_arch                                  = var.lambda_arch
  additional_environment_variables             = local.lambda_environment_variables
  additional_lambda_execution_policy_documents = local.lambda_execution_policies
  lambda_layer_arns                            = local.lambda_layer_arns

  grants_source_data_bucket_name = module.grants_source_data_bucket.bucket_id
  s3_temporary_path_prefix       = local.source_data_bucket_temp_storage_path_prefix

  depends_on = [
    module.grants_source_data_bucket,
  ]
}

module "PublishGrantEvents" {
  source = "./modules/PublishGrantEvents"

  namespace                                    = var.namespace
  function_name                                = "PublishGrantEvents"
  permissions_boundary_arn                     = local.permissions_boundary_arn
  lambda_artifact_bucket                       = module.lambda_artifacts_bucket.bucket_id
  log_retention_in_days                        = var.lambda_default_log_retention_in_days
  log_level                                    = var.lambda_default_log_level
  lambda_autobuild                             = var.lambda_binaries_autobuild
  lambda_binaries_base_path                    = local.lambda_binaries_base_path
  lambda_arch                                  = var.lambda_arch
  additional_environment_variables             = local.lambda_environment_variables
  additional_lambda_execution_policy_documents = local.lambda_execution_policies
  lambda_layer_arns                            = local.lambda_layer_arns

  dynamodb_table_name = module.grants_prepared_dynamodb_table.table_name

  depends_on = [
    module.grants_prepared_dynamodb_table
  ]
}
