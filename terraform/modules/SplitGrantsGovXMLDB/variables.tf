// Common
variable "namespace" {
  type        = string
  description = "Prefix to use for resource names and identifiers."
}

variable "function_name" {
  description = "Name of this Lambda function (excluding namespace prefix)."
  type        = string
}

variable "permissions_boundary_arn" {
  description = "ARN of the IAM policy to apply as a permissions boundary when provisioning a new role. Ignored if `role_arn` is null."
  type        = string
  default     = null
}

variable "lambda_layer_arns" {
  description = "Lambda layer ARNs to attach to the function."
  type        = list(string)
  default     = []
}

variable "lambda_artifact_bucket" {
  description = "Name of the S3 bucket used to store Lambda source artifacts."
  type        = string
}

variable "lambda_binaries_base_path" {
  description = "Path to the local directory where compiled handlers are outputted to per-Lambda subdirectories."
  type        = string
}

variable "lambda_autobuild" {
  description = "When true, a Lambda handler binary will be compiled when missing or outdated. When false, the compiled Lambda handler binary must already exist under `lambda_binaries_base_path`."
  type        = bool
}

variable "lambda_arch" {
  description = "The target build architecture for Lambda functions (either x86_64 or arm64)."
  type        = string

  validation {
    condition     = var.lambda_arch == "x86_64" || var.lambda_arch == "arm64"
    error_message = "Architecture must be x86_64 or arm64."
  }
}

variable "log_level" {
  description = "Value for the LOG_LEVEL environment variable."
  type        = string
  default     = "INFO"
}

variable "log_retention_in_days" {
  description = "Number of days to retain logs."
  type        = number
  default     = 30
}

variable "additional_lambda_execution_policy_documents" {
  description = "JSON policy document(s) containing permissions to configure for the Lambda function, in addition to any defined by this module."
  type        = list(string)
  default     = []
}

variable "additional_environment_variables" {
  description = "Environment variables to configure for the Lambda function, in addition to any defined by this module."
  type        = map(string)
  default     = {}
}

variable "datadog_custom_tags" {
  description = "Custom tags to configure on the DD_TAGS environment variable."
  type        = map(string)
  default     = {}
}

// Module-specific
variable "grants_source_data_bucket_name" {
  description = "Name of the S3 bucket used to store grants source data."
  type        = string
}

variable "grants_prepared_data_bucket_name" {
  description = "Name of the S3 bucket used to store grants prepared data."
  type        = string
}

variable "grants_prepared_dynamodb_table_name" {
  description = "Name of the DynamoDB table used to provide grants prepared data modification timestamps."
  type        = string
}

variable "grants_prepared_dynamodb_table_arn" {
  description = "ARN of the DynamoDB table used to provide grants prepared data modification timestamps."
  type        = string
}

variable "is_forecasted_grants_enabled" {
  description = "Flag to control whether forecasted grants should be processed and stored in S3."
  type        = bool
  default     = false
}

variable "max_split_records" {
  description = "Optional limit (i.e. for testing) on the number of records that the handler will process during a single invocation. This setting is a hard cap on top of opportunity- and forecast-specific limits."
  type        = number
  default     = -1
}

variable "max_split_opportunity_records" {
  description = "Optional limit (i.e. for testing) on the number of opportunity records that the handler will process during a single invocation."
  type        = number
  default     = -1
}

variable "max_split_forecast_records" {
  description = "Optional limit (i.e. for testing) on the number of opportunity records that the handler will process during a single invocation."
  type        = number
  default     = -1
}
