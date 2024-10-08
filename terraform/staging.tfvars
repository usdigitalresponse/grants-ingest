namespace                             = "grants_ingest"
environment                           = "staging"
ssm_deployment_parameters_path_prefix = "/grants_ingest/deploy-config"
datadog_enabled                       = true
lambda_binaries_autobuild             = false
lambda_default_log_retention_in_days  = 30
lambda_default_log_level              = "INFO"
datadog_draft                         = true
datadog_monitors_enabled              = true
ffis_ingest_email_address             = "ffis-ingest@staging.grants.usdr.dev"

// Only defined in staging
datadog_metrics_metadata = {
  "DownloadFFISSpreadsheet.source_size" = {
    short_name  = "Source file size in bytes"
    description = "Size (in bytes) of the downloaded ffis.org spreadsheet file."
    unit        = "byte"
    per_unit    = "file"
  }

  "DownloadGrantsGovDB.source_size" = {
    short_name  = "Source file size in bytes"
    description = "Size (in bytes) of the downloaded grants.gov database archive file."
    unit        = "byte"
    per_unit    = "file"
  }

  "ExtractGrantsGovDBToXML.archive.downloaded" = {
    short_name  = "Downloaded archive files"
    description = "Count of downloaded Grants.gov DB zip archives for extraction."
    unit        = "file"
  }

  "ExtractGrantsGovDBToXML.xml.extracted" = {
    short_name  = "Extracted XML files"
    description = "Count of XML files extracted from the Grants.gov DB zip archive."
    unit        = "file"
  }

  "ExtractGrantsGovDBToXML.xml.uploaded" = {
    short_name  = "Uploaded XML files"
    description = "Count of uploaded XML files after extraction."
    unit        = "file"
  }

  "PersistFFISData.opportunity.saved" = {
    short_name  = "Saved opportunities"
    description = "Count of opportunity records persisted to DynamoDB with FFIS.org data."
    unit        = "record"
  }

  "PersistGrantsGovXMLDB.record.saved" = {
    short_name  = "Saved grant records"
    description = "Count of grant records persisted to DynamoDB with Grants.gov data."
    unit        = "record"
  }

  "PersistGrantsGovXMLDB.record.failed" = {
    short_name  = "Failed grant records"
    description = "Count of grant records that failed to be persisted to DynamoDB with Grants.gov data."
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

  "ReceiveFFISEmail.email.untrusted" = {
    short_name  = "Received untrusted email"
    description = "Count of received emails that were determined to be untrustworthy."
    unit        = "email"
  }

  "SplitFFISSpreadsheet.opportunity.created" = {
    short_name  = "New grant opportunities"
    description = "Count of new grant opportunity records created from FFIS.org data during invocation."
    unit        = "record"
  }

  "SplitFFISSpreadsheet.opportunity.failed" = {
    short_name  = "Failed grant opportunities"
    description = "Count of grant opportunity records from Grants.gov data that failed to process during invocation."
    unit        = "record"
  }

  "SplitFFISSpreadsheet.spreadsheet.row_count" = {
    short_name  = "Spreadsheet row count"
    description = "Number of rows contained in source spreadsheets from FFIS.org."
    unit        = "row"
  }

  "SplitFFISSpreadsheet.cell_parsing_errors" = {
    short_name  = "Spreadsheet cell parsing errors"
    description = "Count of parsing errors encountered in source spreadsheets from FFIS.org."
    unit        = "error"
  }

  "SplitGrantsGovXMLDB.record.created" = {
    short_name  = "New grant records"
    description = "Count of new grant records created from Grants.gov data during invocation."
    unit        = "record"
  }

  "SplitGrantsGovXMLDB.record.updated" = {
    short_name  = "Updated grant records"
    description = "Count of modified grant records updated from Grants.gov data during invocation."
    unit        = "record"
  }

  "SplitGrantsGovXMLDB.record.skipped" = {
    short_name  = "Skipped grant records"
    description = "Count of unchanged grant records from Grants.gov data skipped during invocation."
    unit        = "record"
  }

  "SplitGrantsGovXMLDB.record.failed" = {
    short_name  = "Failed grant records"
    description = "Count of grant records from Grants.gov data that failed to process during invocation."
    unit        = "record"
  }
}
