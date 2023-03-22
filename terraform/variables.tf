variable "enabled" {
  type        = bool
  description = "When false, disables resource creation."
  default     = true
}

variable "namespace" {
  type        = string
  description = "Prefix to use for resource names and identifiers."
}

variable "environment" {
  type        = string
  description = "Name of the environment/stage targeted by deployments (e.g. sandbox/staging/prod)."
}

variable "version_identifier" {
  type        = string
  description = "The version for this service deployment."
}

variable "permissions_boundary_policy_name" {
  description = "Name of the permissions boundary for service roles"
  type        = string
  default     = "service-management-boundary"
}

variable "tags" {
  type    = map(string)
  default = {}
}

variable "ssm_deployment_parameters_path_prefix" {
  type        = string
  description = "Base path for all SSM parameters used for deployment."
  validation {
    condition     = startswith(var.ssm_deployment_parameters_path_prefix, "/")
    error_message = "Value must start with a forward slash."
  }
  validation {
    condition     = !endswith(var.ssm_deployment_parameters_path_prefix, "/")
    error_message = "Value cannot end with a trailing slash."
  }
}

variable "lambda_code_path" {
  description = "Path to the base source code directory for this project."
  type        = string
  default     = ""
}

variable "lambda_arch" {
  description = "The target build architecture for Lambda functions (either x86_64 or arm64)."
  type        = string
  default     = "arm64"

  validation {
    condition     = var.lambda_arch == "x86_64" || var.lambda_arch == "arm64"
    error_message = "Architecture must be x86_64 or arm64."
  }
}

variable "datadog_enabled" {
  description = "Whether to enable datadog instrumentation in the current environment."
  type        = bool
  default     = false
}

variable "additional_lambda_environment_variables" {
  description = "Map of additional/override environment variables to apply to all Lambda functions."
  type        = map(string)
  default     = {}
}

variable "datadog_tags" {
  description = "Datadog reserved tags to configure in Lambda function environments (when var.datadog_enabled is true)."
  type        = map(string)
}

variable "lambda_default_log_retention_in_days" {
  description = "Default number of days to retain Lambda execution logs."
  type        = number
  default     = 30
}

variable "lambda_default_log_level" {
  description = "Default logging level to configure (as LOG_LEVEL env var) on Lambda functions."
  type        = string
  default     = "INFO"
}

variable "eventbridge_scheduler_enabled" {
  description = "If false, uses CloudWatch Events to schedule Lambda execution. This should only be false in local development."
  type        = bool
  default     = true
}
