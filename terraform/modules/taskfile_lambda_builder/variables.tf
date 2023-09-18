variable "s3_bucket" {
  description = "Name of the S3 bucket in which Lambda artifacts will be stored."
  type        = string
}

variable "s3_key_prefix" {
  description = "Prefix for Lambda zip artifact S3 object keys."
  type        = string
  default     = ""
}

variable "binary_base_path" {
  description = "Path to directory where Taskfile-managed builds output per-Lambda directories."
  type        = string
}

variable "function_name" {
  description = "Name of this Lambda function, consistent with Taskfile commands and outputs."
  type        = string
}

variable "override_artifact_base_path" {
  description = "Path to directory where this module should output Lambda zip artifacts. Uses terraform project root by default."
  type        = string
  default     = null
}

variable "override_taskfile_command" {
  description = "The Taskfile command used to compile the Lambda handler binary. Uses 'build-<function_name>' by default."
  type        = string
  default     = null
}

variable "autobuild" {
  description = "Whether to issue a Taskfile command to compile the Lambda handler binary when missing or outdated. When false, only a preexisting binary will be used. Recommendation: 'true' for development; 'false' for CI/CD."
  type        = bool
  default     = true
}

variable "override_path_to_binary" {
  description = "Explicit path to the file (outputted by the Taskfile command) that will be zipped. Uses '<binary_base_path>/<function_name>/bootstrap' by default."
  type        = string
  default     = null
}
