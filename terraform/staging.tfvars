namespace                             = "grants_ingest-staging"
environment                           = "staging"
ssm_deployment_parameters_path_prefix = "/grants_ingest/staging/deploy-config"
datadog_enabled                       = true
lambda_default_log_retention_in_days  = 30
lambda_default_log_level              = "INFO"
datadog_draft                         = true
ffis_ingest_email_address             = "ffis-ingest@staging.grants.usdr.dev"

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

  "PublishGrantEvents.invocation_batch_size" = {
    short_name  = "Invocation batch size"
    description = "Count of records contained in the stream batch for an invocation."
    unit        = "record"
  }

  "PublishGrantEvents.record.failed" = {
    short_name  = "Failed invocation batch records"
    description = "Count of records in the stream invocation batch that did not result in a published event."
    unit        = "record"
  }

  "PublishGrantEvents.event.published" = {
    short_name  = "Published events"
    description = "Count of events published to EventBridge."
    unit        = "message"
  }

  "PublishGrantEvents.item_image.build" = {
    short_name  = "Item image build attempts"
    description = "Count of attempts to build a data document mapped from a DynamoDB item image."
    unit        = "attempt"
  }

  "PublishGrantEvents.item_image.unbuildable" = {
    short_name  = "Unbuildable item images"
    description = "Count of failed attempts to build a data document mapped from a DynamoDB item image."
    unit        = "attempt"
  }

  "PublishGrantEvents.item_image.malformatted_field" = {
    short_name  = "Malformatted item image fields"
    description = "Count of DynamoDB item image attributes found to be incompatible with the target schema."
    unit        = "occurrence"
  }

  "PublishGrantEvents.grant_data.invalid" = {
    short_name  = "Invalid mapper results"
    description = "Count of grants mapped from a DynamoDB item image that failed target schema validation."
    unit        = "document"
  }
}
