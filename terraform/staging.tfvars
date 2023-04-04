namespace                             = "grants_ingest-staging"
environment                           = "staging"
ssm_deployment_parameters_path_prefix = "/grants_ingest/staging/deploy-config"
datadog_enabled                       = true
lambda_default_log_retention_in_days  = 30
lambda_default_log_level              = "INFO"
datadog_draft                         = true

// Only defined in staging
datadog_metrics_metadata = {
  "DownloadGrantsGovDB.source_size" = {
    short_name  = "Source file size in bytes"
    description = "Size (in bytes) of the downloaded grants.gov database archive file."
    unit        = "byte"
    per_unit    = "file"
  }

  "SplitGrantsGovXMLDB.opportunity.created" = {
    short_name  = "New grant opportunities"
    description = "Count of new grant opportunity records created during invocation."
    unit        = "record"
  }

  "SplitGrantsGovXMLDB.opportunity.updated" = {
    short_name  = "Updated grant opportunities"
    description = "Count of modified grant opportunity records updated during invocation."
    unit        = "record"
  }

  "SplitGrantsGovXMLDB.opportunity.skipped" = {
    short_name  = "Skipped grant opportunities"
    description = "Count of unchanged grant opportunity records skipped during invocation."
    unit        = "record"
  }

  "SplitGrantsGovXMLDB.opportunity.failed" = {
    short_name  = "Failed grant opportunities"
    description = "Count of grant opportunity records that failed to process during invocation."
    unit        = "record"
  }
}
