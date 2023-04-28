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

variable "git_repository_url" {
  type        = string
  description = "URL for the repository that provides this service."
  default     = "github.com/usdigitalresponse/grants-ingest"
}

variable "git_commit_sha" {
  type        = string
  description = "Git commit SHA for which terraform is being deployed."
  default     = ""
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

variable "additional_lambda_environment_variables" {
  description = "Map of additional/override environment variables to apply to all Lambda functions."
  type        = map(string)
  default     = {}
}

variable "datadog_enabled" {
  description = "Whether to enable datadog instrumentation in the current environment."
  type        = bool
  default     = false
}

variable "datadog_dashboards_enabled" {
  description = "Whether to provision Datadog dashboards."
  type        = bool
  default     = true
}

variable "datadog_api_key" {
  description = "API key to use when provisioning Datadog resources."
  type        = string
  default     = ""
  sensitive   = true
}

variable "datadog_app_key" {
  description = "Application key to use when provisioning Datadog resources."
  type        = string
  default     = ""
  sensitive   = true
}

variable "datadog_draft" {
  description = "Marks datadog resources as drafts. Set to false unless deploying to Production."
  type        = bool
  default     = false
}

variable "datadog_reserved_tags" {
  description = "Datadog reserved tags to configure in Lambda function environments (when var.datadog_enabled is true)."
  type        = map(string)
  default     = {}

  validation {
    condition = alltrue([
      for k in keys(var.datadog_reserved_tags) :
      contains(["DD_ENV", "DD_SERVICE", "DD_VERSION"], k)
    ])
    error_message = "Datadog reserved tags may only include keys DD_ENV, DD_SERVICE, or DD_VERSION."
  }
}

variable "datadog_lambda_custom_tags" {
  description = "Custom (non-reserved) tags for configuring on the DD_TAGS environment variable for all Lambda functions."
  type        = map(string)
  default     = {}

  validation {
    condition = !anytrue([
      for k in keys(var.datadog_lambda_custom_tags) :
      contains(["DD_ENV", "DD_SERVICE", "DD_VERSION"], upper(k))
    ])
    error_message = "Datadog reserved tags may not be configured with this variable (see var.datadog_reserved_tags)."
  }

  validation {
    condition     = alltrue([for k in keys(var.datadog_lambda_custom_tags) : (k == lower(k))])
    error_message = "Datadog custom tag keys must be lowercase."
  }

  validation {
    condition     = alltrue([for v in values(var.datadog_lambda_custom_tags) : (v == lower(v))])
    error_message = "Datadog custom tag values must be lowercase."
  }
}

variable "datadog_lambda_extension_version" {
  description = "Version to use for the Datadog Lambda Extension layer (when var.datadog_enabled is true)."
  type        = string
  default     = "41"
}

variable "datadog_metrics_metadata" {
  description = "Map of metadata describing custom Datadog metrics, keyed by the metric name. All metrics are automatically prefixed with grants_ingest."
  type = map(object({
    short_name  = optional(string)
    description = optional(string)
    unit        = optional(string) # https://docs.datadoghq.com/metrics/units/
    per_unit    = optional(string)
  }))
  default = {}
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

variable "ffis_ingest_email_address" {
  type        = string
  description = "Email address used to receive FFIS digests and save them to S3"
  default     = "ffis-ingest@grants.usdigitalresponse.org"
}

variable "ses_active_receipt_rule_set_enabled" {
  description = "If false, uses CloudWatch Events to schedule Lambda execution. This should only be false in local development."
  type        = bool
  default     = true
}