namespace                             = "grants_ingest"
environment                           = "production"
ssm_deployment_parameters_path_prefix = "/grants_ingest/production/deploy-config"
lambda_binaries_autobuild             = false
lambda_default_log_retention_in_days  = 30
lambda_default_log_level              = "INFO"
ffis_ingest_email_address             = "ffis-ingest@grants.usdigitalresponse.org"

datadog_enabled          = true
datadog_draft            = false
datadog_monitors_enabled = true
datadog_monitor_notification_handles = [
  "thendrickson@usdigitalresponse.org",
  "asridhar@usdigitalresponse.org",
]
