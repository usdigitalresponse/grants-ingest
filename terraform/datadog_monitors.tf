locals {
  dd_monitor_default_evaluation_delay = 900
  dd_monitor_default_tags             = ["service:grants-ingest", "team:grants"]
  dd_monitor_default_notify_handles = [
    "slack-project-gost-auto-notifs",
    "thendrickson@usdigitalresponse.org",
    "asridhar@usdigitalresponse.org",
  ]
  dd_monitor_default_notify = join(" ", [
    for v in local.dd_monitor_default_notify_handles : "@${v}"
  ])
}

resource "datadog_monitor" "events_failed_to_publish" {
  name = "Grants Ingest: Grant modificaiton events failed to publish"
  type = "metric alert"
  message = join("\n", [
    "{{#is_alert}}",
    "Alert: The PublishGrantEvents step was unable to publish one or more grant modifications received from DynamoDB.",
    "Investigate the issue and once resolved, re-trigger the step with the most-recent revision of the grant(s) associated with the failure.",
    "This monitor will not return to normal while there are messages in the DLQ.",
    "{{/is_alert}}",
    "{{#is_recovery}}",
    "Recovery: There are no longer messages in the DLQ.",
    "{{/is_recovery}}",
    "Notify: ${local.dd_monitor_default_notify}",
  ])

  query = "min(last_1h):avg:aws.sqs.approximate_number_of_messages_visible{env:production,queuename:${module.PublishGrantEvents.dlq_name}} > 0"

  notify_no_data   = false
  evaluation_delay = local.dd_monitor_default_evaluation_delay
  monitor_thresholds = {
    critical          = 1
    critical_recovery = 0
  }

  tags = local.dd_monitor_default_tags
}

resource "datadog_monitor" "DownloadGrantsGovDB-failed" {
  name = "Grants Ingest: DownloadGrantsGovDB failed"
  type = "metric alert"
  message = join("\n", [
    "Alert: Cannot download grants database archive from grants.gov.",
    "This may be due to a temporary service outage on grants.gov.",
    "Verify whether a download is available. If it is, investigate the cause of this failure,",
    "and then trigger a new download attempt.",
    "Notify: ${local.dd_monitor_default_notify}",
  ])

  query = "sum(last_10h):avg:aws.lambda.errors{env:production,handlername:downloadgrantsgovdb}.as_count() > 0"

  notify_no_data   = false
  evaluation_delay = local.dd_monitor_default_evaluation_delay
  monitor_thresholds = {
    critical = 1
  }

  tags = local.dd_monitor_default_tags
}

resource "datadog_monitor" "DownloadFFISSpreadsheet-failed" {
  name = "Grants Ingest: DownloadFFISSpreadsheet failed"
  type = "metric alert"
  message = join("\n", [
    "Alert: Cannot download spreadsheet from FFIS.",
    "Notify: ${local.dd_monitor_default_notify}",
  ])

  query = "sum(last_1h):avg:aws.lambda.errors{env:production,handlername:downloadffisspreadsheet}.as_count() > 0"

  notify_no_data   = false
  evaluation_delay = local.dd_monitor_default_evaluation_delay
  monitor_thresholds = {
    critical = 1
  }

  tags = local.dd_monitor_default_tags
}

resource "datadog_monitor" "EnqueueFFISSpreadsheet-failed" {
  name = "Grants Ingest: EnqueueFFISSpreadsheet failed"
  type = "metric alert"
  message = join("\n", [
    "Alert: Failed when attempting to enqueue download for FFIS spreadsheet link parsed from email.",
    "Notify: ${local.dd_monitor_default_notify}",
  ])

  query = "sum(last_1h):avg:aws.lambda.errors{env:production,handlername:enqueueffisspreadsheet}.as_count() > 0"

  notify_no_data   = false
  evaluation_delay = local.dd_monitor_default_evaluation_delay
  monitor_thresholds = {
    critical = 1
  }

  tags = local.dd_monitor_default_tags
}

resource "datadog_monitor" "ExtractGrantsGovXMLToDB-failed" {
  name = "Grants Ingest: ExtractGrantsGovXMLToDB failed"
  type = "metric alert"
  message = join("\n", [
    "Failed to extract XML from Grants.gov zip archive.",
    "Notify: ${local.dd_monitor_default_notify}",
  ])

  query = "sum(last_1h):avg:aws.lambda.errors{env:production,handlername:enqueueffisspreadsheet}.as_count() > 0"

  notify_no_data   = false
  evaluation_delay = local.dd_monitor_default_evaluation_delay
  monitor_thresholds = {
    critical = 1
  }

  tags = local.dd_monitor_default_tags
}

resource "datadog_monitor" "PersistFFISData-failed" {
  name = "Grants Ingest: PersistFFISData failed"
  type = "metric alert"
  message = join("\n", [
    "Alert: Failed to save FFIS data to DynamoDB.",
    "Notify: ${local.dd_monitor_default_notify}",
  ])

  query = "sum(last_1h):avg:aws.lambda.errors{env:production,handlername:persistffisdata}.as_count() > 0"

  notify_no_data   = false
  evaluation_delay = local.dd_monitor_default_evaluation_delay
  monitor_thresholds = {
    critical = 1
  }

  tags = local.dd_monitor_default_tags
}

resource "datadog_monitor" "PersistGrantsGovXMLDB-failed" {
  name = "Grants Ingest: PersistGrantsGovXMLDB failed"
  type = "metric alert"
  message = join("\n", [
    "Alert: Failed to save Grants.gov data to DynamoDB.",
    "Notify: ${local.dd_monitor_default_notify}",
  ])

  query = "sum(last_1h):avg:aws.lambda.errors{env:production,handlername:persistgrantsgovxmldb}.as_count() > 0"

  notify_no_data   = false
  evaluation_delay = local.dd_monitor_default_evaluation_delay
  monitor_thresholds = {
    critical = 1
  }

  tags = local.dd_monitor_default_tags
}

resource "datadog_monitor" "ReceiveFFISEmail-failed" {
  name = "Grants Ingest: ReceiveFFISEmail failed"
  type = "metric alert"
  message = join("\n", [
    "Alert: There was a problem with an email delivered to the FFIS inbox.",
    "This may be due to spam/virus detection or an unrecognized sender.",
    "Notify: ${local.dd_monitor_default_notify}",
  ])

  query = "sum(last_1h):avg:aws.lambda.errors{env:production,handlername:receiveffisemail}.as_count() > 0"

  notify_no_data   = false
  evaluation_delay = local.dd_monitor_default_evaluation_delay
  monitor_thresholds = {
    critical = 1
  }

  tags = local.dd_monitor_default_tags
}

resource "datadog_monitor" "SplitFFISSpreadsheet-failed" {
  name = "Grants Ingest: SplitFFISSpreadsheet failed"
  type = "metric alert"
  message = join("\n", [
    "Alert: A failure occurred while attempting to parse data from an FFIS spreadsheet.",
    "Notify: ${local.dd_monitor_default_notify}",
  ])

  query = "sum(last_1h):avg:aws.lambda.errors{env:production,handlername:splitffisspreadsheet}.as_count() > 0"

  notify_no_data   = false
  evaluation_delay = local.dd_monitor_default_evaluation_delay
  monitor_thresholds = {
    critical = 1
  }

  tags = local.dd_monitor_default_tags
}

resource "datadog_monitor" "SplitGrantsGovXMLDB-failed" {
  name = "Grants Ingest: SplitGrantsGovXMLDB failed"
  type = "metric alert"
  message = join("\n", [
    "Alert: A failure occurred while attempting to parse data from a Grants.gov XML file.",
    "Notify: ${local.dd_monitor_default_notify}",
  ])

  query = "sum(last_1h):avg:aws.lambda.errors{env:production,handlername:splitgrantsgovxmldb}.as_count() > 0"

  notify_no_data   = false
  evaluation_delay = local.dd_monitor_default_evaluation_delay
  monitor_thresholds = {
    critical = 1
  }

  tags = local.dd_monitor_default_tags
}

resource "datadog_monitor" "SplitGrantsGovXMLDB-no_opportunities_created" {
  name = "Grants Ingest: SplitGrantsGovXMLDB has not created new grant opportunities"
  type = "metric alert"
  message = join("\n", [
    "Alert: No new grant opportunities have been created from Grants.gov data in the past 4 days.",
    "While it is possible that new opportunities have not been published, it is unusual",
    "and likely indicates a problem with SplitGrantsGovXMLDB or an earlier pipeline step.",
    "Notify: ${local.dd_monitor_default_notify}",
  ])

  query = "sum(last_4d):sum:grants_ingest.SplitGrantsGovXMLDB.opportunity.created{env:production}.as_count() < 1"

  notify_no_data   = false
  evaluation_delay = local.dd_monitor_default_evaluation_delay
  monitor_thresholds = {
    critical = 1
  }

  tags = local.dd_monitor_default_tags
}

resource "datadog_monitor" "SplitFFISSpreadsheet-no_opportunities_created" {
  name = "Grants Ingest: SplitFFISSpreadsheet has not created new grant opportunities"
  type = "metric alert"
  message = join("\n", [
    "Alert: No new grant opportunities have been created from FFIS data in the past 9 days.",
    "While it is possible that new opportunities have not been published, it is unusual",
    "and likely indicates a problem with SplitFFISSpreadsheet or an earlier pipeline step.",
    "Notify: ${local.dd_monitor_default_notify}",
  ])

  query = "sum(last_4d):sum:grants_ingest.SplitFFISSpreadsheet.opportunity.created{env:production}.as_count() < 1"

  notify_no_data   = false
  evaluation_delay = local.dd_monitor_default_evaluation_delay
  monitor_thresholds = {
    critical = 1
  }

  tags = local.dd_monitor_default_tags
}

resource "datadog_monitor" "PublishGrantEvents-no_events_published" {
  name = "Grants Ingest: PublishGrantEvents has not published any events in a while"
  type = "metric alert"
  message = join("\n", [
    "Alert: No grant modification events have been published in the past 4 days.",
    "While it is possible that new opportunities have not been created or modified,",
    "it likely indicates a problem with PublishGrantEvents or an earlier pipeline step.",
    "Notify: ${local.dd_monitor_default_notify}",
  ])

  query = "sum(last_4d):sum:grants_ingest.PublishGrantEvents.event.published{env:production}.as_count() < 1"

  notify_no_data   = false
  evaluation_delay = local.dd_monitor_default_evaluation_delay
  monitor_thresholds = {
    critical = 1
  }

  tags = local.dd_monitor_default_tags
}
