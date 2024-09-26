locals {
  datadog_draft_label = var.datadog_draft ? "(Draft - ${var.environment})" : ""
  datadog_notes_dir   = "${path.module}/datadog_notes"
}

resource "datadog_dashboard" "service_dashboard" {
  count = var.datadog_dashboards_enabled ? 1 : 0

  title       = trimspace("Grants Ingest Service Dashboard ${local.datadog_draft_label}")
  description = "Dashboard for monitoring the Grants Ingest pipeline service."
  layout_type = "ordered"
  reflow_type = "fixed"

  template_variable {
    name     = "env"
    prefix   = "env"
    defaults = ["production"]
  }

  template_variable {
    name             = "service"
    prefix           = "service"
    available_values = ["grants-ingest"]
    defaults         = ["grants-ingest"]
  }

  template_variable {
    name     = "version"
    prefix   = "version"
    defaults = ["*"]
  }

  // Service Summary
  widget {
    group_definition {
      title       = "Service Summary"
      show_title  = true
      layout_type = "ordered"

      widget {
        trace_service_definition {
          title              = "Overview"
          env                = "$env"
          service            = "$service"
          span_name          = "aws.lambda"
          display_format     = "two_column"
          size_format        = "large"
          show_breakdown     = false
          show_distribution  = false
          show_errors        = true
          show_hits          = true
          show_latency       = false
          show_resource_list = true
        }
        widget_layout {
          x      = 0
          y      = 0
          width  = 8
          height = 9
        }
      }

      widget {
        manage_status_definition {
          title               = "Monitors"
          query               = "tag:($env AND $service)"
          display_format      = "countsAndList"
          color_preference    = "text"
          hide_zero_counts    = true
          show_priority       = false
          show_last_triggered = true
          sort                = "status,asc"
        }
        widget_layout {
          x      = 8
          y      = 0
          width  = 4
          height = 6
        }
      }

      widget {
        sunburst_definition {
          title      = "Grant Events Published in Period"
          hide_total = false

          request {
            formula {
              formula_expression = "published_by_type"
            }
            query {
              metric_query {
                name        = "published_by_type"
                data_source = "metrics"
                query       = "sum:grants_ingest.PublishGrantEvents.event.published{$env,$service} by {type}.as_count()"
                aggregator  = "sum"
              }
            }
          }
          legend_inline {
            hide_percent = false
            hide_value   = false
            type         = "inline"
          }
        }
        widget_layout {
          x      = 8
          y      = 6
          width  = 4
          height = 3
        }
      }
    }
    widget_layout {
      x      = 0
      y      = 0
      width  = 12
      height = 9
    }
  }

  // DownloadFFISSpreadsheet
  widget {
    group_definition {
      title       = "DownloadFFISSpreadsheet"
      show_title  = true
      layout_type = "ordered"

      widget {
        timeseries_definition {
          title          = "Invocation Status"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            formula {
              formula_expression = "invoke_total - invoke_failure"
              alias              = "Succeeded"
            }
            query {
              metric_query {
                name  = "invoke_total"
                query = "sum:aws.lambda.invocations{$env,$service,$version,functionname:${lower(module.DownloadFFISSpreadsheet.lambda_function_name)}}.as_count()"
              }
            }

            formula {
              formula_expression = "invoke_failure"
              alias              = "Failed"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_failure"
                query = "sum:aws.lambda.errors{$env,$service,$version,functionname:${lower(module.DownloadFFISSpreadsheet.lambda_function_name)}}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 0
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Invocation Duration"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "line"

            formula {
              formula_expression = "invoke_duration"
              alias              = "Duration"
            }
            query {
              metric_query {
                name  = "invoke_duration"
                query = "avg:aws.lambda.duration{$env,$service,$version,functionname:${lower(module.DownloadFFISSpreadsheet.lambda_function_name)}}"
              }
            }

            formula {
              formula_expression = "invoke_timeout"
              alias              = "Timeout"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_timeout"
                query = "avg:aws.lambda.timeout{$env,$service,$version,functionname:${lower(module.DownloadFFISSpreadsheet.lambda_function_name)}}"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "FFIS.org Spreadsheet File Size"
          legend_layout  = "horizontal"
          legend_columns = ["value"]
          show_legend    = true

          yaxis {
            include_zero = false
          }

          request {
            display_type = "line"

            formula {
              formula_expression = "size"
              alias              = "Size"
            }
            query {
              metric_query {
                name  = "size"
                query = "avg:grants_ingest.DownloadFFISSpreadsheet.source_size{$env,$service,$version}"
              }
            }
          }
        }
        widget_layout {
          x      = 8
          y      = 0
          width  = 4
          height = 3
        }
      }
    }
    widget_layout {
      x      = 0
      y      = 9
      width  = 12
      height = 3
    }
  }

  // DownloadGrantsGovDB
  widget {
    group_definition {
      title       = "DownloadGrantsGovDB"
      show_title  = true
      layout_type = "ordered"

      widget {
        timeseries_definition {
          title          = "Invocation Status"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            formula {
              formula_expression = "invoke_total - invoke_failure"
              alias              = "Succeeded"
            }
            query {
              metric_query {
                name  = "invoke_total"
                query = "sum:aws.lambda.invocations{$env,$service,$version,functionname:${lower(module.DownloadGrantsGovDB.lambda_function_name)}}.as_count()"
              }
            }

            formula {
              formula_expression = "invoke_failure"
              alias              = "Failed"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_failure"
                query = "sum:aws.lambda.errors{$env,$service,$version,functionname:${lower(module.DownloadGrantsGovDB.lambda_function_name)}}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 0
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Invocation Duration"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "line"

            formula {
              formula_expression = "invoke_duration"
              alias              = "Duration"
            }
            query {
              metric_query {
                name  = "invoke_duration"
                query = "avg:aws.lambda.duration{$env,$service,$version,functionname:${lower(module.DownloadGrantsGovDB.lambda_function_name)}}"
              }
            }

            formula {
              formula_expression = "invoke_timeout"
              alias              = "Timeout"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_timeout"
                query = "avg:aws.lambda.timeout{$env,$service,$version,functionname:${lower(module.DownloadGrantsGovDB.lambda_function_name)}}"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Grants.gov DB Zip File Size"
          legend_layout  = "horizontal"
          legend_columns = ["value"]
          show_legend    = true

          yaxis {
            include_zero = false
          }

          request {
            display_type = "line"

            formula {
              formula_expression = "size"
              alias              = "Size"
            }
            query {
              metric_query {
                name  = "size"
                query = "avg:grants_ingest.DownloadGrantsGovDB.source_size{$env,$service,$version}"
              }
            }
          }
        }
        widget_layout {
          x      = 8
          y      = 0
          width  = 4
          height = 3
        }
      }
    }
    widget_layout {
      x      = 0
      y      = 12
      width  = 12
      height = 3
    }
  }

  // EnqueueFFISDownload
  widget {
    group_definition {
      title       = "EnqueueFFISDownload"
      show_title  = true
      layout_type = "ordered"

      widget {
        timeseries_definition {
          title          = "Invocation Status"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            formula {
              formula_expression = "invoke_total - invoke_failure"
              alias              = "Succeeded"
            }
            query {
              metric_query {
                name  = "invoke_total"
                query = "sum:aws.lambda.invocations{$env,$service,$version,functionname:${lower(module.EnqueueFFISDownload.lambda_function_name)}}.as_count()"
              }
            }

            formula {
              formula_expression = "invoke_failure"
              alias              = "Failed"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_failure"
                query = "sum:aws.lambda.errors{$env,$service,$version,functionname:${lower(module.EnqueueFFISDownload.lambda_function_name)}}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 0
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Invocation Duration"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "line"

            formula {
              formula_expression = "invoke_duration"
              alias              = "Duration"
            }
            query {
              metric_query {
                name  = "invoke_duration"
                query = "avg:aws.lambda.duration{$env,$service,$version,functionname:${lower(module.EnqueueFFISDownload.lambda_function_name)}}"
              }
            }

            formula {
              formula_expression = "invoke_timeout"
              alias              = "Timeout"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_timeout"
                query = "avg:aws.lambda.timeout{$env,$service,$version,functionname:${lower(module.EnqueueFFISDownload.lambda_function_name)}}"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 0
          width  = 4
          height = 3
        }
      }

      // Shows queue size
      widget {
        timeseries_definition {
          title          = "Queue Size"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          yaxis {
            include_zero = false
            scale        = "sqrt"
          }

          request {
            display_type = "bars"

            formula {
              formula_expression = "queue_size"
              alias              = "Messages"
            }
            query {
              metric_query {
                name  = "queue_size"
                query = "sum:aws.sqs.approximate_number_of_messages_visible{$env,$service,$version,queuename:${lower(aws_sqs_queue.ffis_downloads.name)}}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 8
          y      = 0
          width  = 4
          height = 3
        }
      }
    }
    widget_layout {
      x      = 0
      y      = 15
      width  = 12
      height = 3
    }
  }

  // ExtractGrantsGovDBToXML
  widget {
    group_definition {
      title       = "ExtractGrantsGovDBToXML"
      show_title  = true
      layout_type = "ordered"

      widget {
        timeseries_definition {
          title          = "Invocation Status"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            formula {
              formula_expression = "invoke_total - invoke_failure"
              alias              = "Succeeded"
            }
            query {
              metric_query {
                name  = "invoke_total"
                query = "sum:aws.lambda.invocations{$env,$service,$version,functionname:${lower(module.ExtractGrantsGovDBToXML.lambda_function_name)}}.as_count()"
              }
            }

            formula {
              formula_expression = "invoke_failure"
              alias              = "Failed"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_failure"
                query = "sum:aws.lambda.errors{$env,$service,$version,functionname:${lower(module.ExtractGrantsGovDBToXML.lambda_function_name)}}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 0
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Invocation Duration"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "line"

            formula {
              formula_expression = "invoke_duration"
              alias              = "Duration"
            }
            query {
              metric_query {
                name  = "invoke_duration"
                query = "avg:aws.lambda.duration{$env,$service,$version,functionname:${lower(module.ExtractGrantsGovDBToXML.lambda_function_name)}}"
              }
            }

            formula {
              formula_expression = "invoke_timeout"
              alias              = "Timeout"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_timeout"
                query = "avg:aws.lambda.timeout{$env,$service,$version,functionname:${lower(module.ExtractGrantsGovDBToXML.lambda_function_name)}}"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Extraction Results"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            formula {
              formula_expression = "archives_downloaded"
              alias              = "Zip Files Downloaded"
              style {
                palette       = "cool"
                palette_index = 4
              }
            }
            query {
              metric_query {
                name  = "archives_downloaded"
                query = "sum:grants_ingest.ExtractGrantsGovDBToXML.archive.downloaded{$env,$service,$version}.as_count()"
              }
            }

            formula {
              formula_expression = "xml_files_extracted"
              alias              = "XML Files Extracted"
              style {
                palette       = "purple"
                palette_index = 4
              }
            }
            query {
              metric_query {
                name  = "xml_files_extracted"
                query = "sum:grants_ingest.ExtractGrantsGovDBToXML.xml.extracted{$env,$service,$version}.as_count()"
              }
            }

            formula {
              formula_expression = "xml_files_uploaded"
              alias              = "XML Files Uploaded"
              style {
                palette       = "classic"
                palette_index = 4
              }
            }
            query {
              metric_query {
                name  = "xml_files_uploaded"
                query = "sum:grants_ingest.ExtractGrantsGovDBToXML.xml.uploaded{$env,$service,$version}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 8
          y      = 0
          width  = 4
          height = 3
        }
      }
    }
    widget_layout {
      x      = 0
      y      = 18
      width  = 12
      height = 3
    }
  }

  // PersistFFISData
  widget {
    group_definition {
      title       = "PersistFFISData"
      show_title  = true
      layout_type = "ordered"

      widget {
        timeseries_definition {
          title          = "Invocation Status"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            formula {
              formula_expression = "invoke_total - invoke_failure"
              alias              = "Succeeded"
            }
            query {
              metric_query {
                name  = "invoke_total"
                query = "sum:aws.lambda.invocations{$env,$service,$version,functionname:${lower(module.PersistFFISData.lambda_function_name)}}.as_count()"
              }
            }

            formula {
              formula_expression = "invoke_failure"
              alias              = "Failed"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_failure"
                query = "sum:aws.lambda.errors{$env,$service,$version,functionname:${lower(module.PersistFFISData.lambda_function_name)}}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 0
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Invocation Duration"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "line"

            formula {
              formula_expression = "invoke_duration"
              alias              = "Duration"
            }
            query {
              metric_query {
                name  = "invoke_duration"
                query = "avg:aws.lambda.duration{$env,$service,$version,functionname:${lower(module.PersistFFISData.lambda_function_name)}}"
              }
            }

            formula {
              formula_expression = "invoke_timeout"
              alias              = "Timeout"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_timeout"
                query = "avg:aws.lambda.timeout{$env,$service,$version,functionname:${lower(module.PersistFFISData.lambda_function_name)}}"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 0
          width  = 4
          height = 3
        }
      }

      // Shows counts of opportunities saved vs skipped due to being unmodified
      widget {
        timeseries_definition {
          title          = "Opportunity Records Saved vs Skipped"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          yaxis {
            include_zero = false
            scale        = "sqrt"
          }

          request {
            display_type = "bars"

            formula {
              formula_expression = "records_saved"
              alias              = "Saved"
              style {
                palette       = "cool"
                palette_index = 4
              }
            }
            query {
              metric_query {
                name  = "records_saved"
                query = "sum:grants_ingest.PersistFFISData.opportunity.saved{$env,$service,$version}.as_count()"
              }
            }

            // Derived skipped count by subtracting invocation errors and saved record counts
            // from the total number of invocations.
            formula {
              formula_expression = "invocation_total - invocation_failure - records_saved"
              alias              = "Skipped (Unmodified)"
              style {
                palette       = "warm"
                palette_index = 4
              }
            }
            query {
              metric_query {
                name  = "records_saved"
                query = "sum:grants_ingest.PersistFFISData.opportunity.saved{$env,$service,$version}.as_count()"
              }
            }
            query {
              metric_query {
                name  = "invocation_total"
                query = "sum:aws.lambda.invocations{$env,$service,$version,functionname:${lower(module.PersistFFISData.lambda_function_name)}}.as_count()"
              }
            }
            query {
              metric_query {
                name  = "invocation_failure"
                query = "sum:aws.lambda.errors{$env,$service,$version,functionname:${lower(module.PersistFFISData.lambda_function_name)}}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 8
          y      = 0
          width  = 4
          height = 3
        }
      }
    }
    widget_layout {
      x      = 0
      y      = 21
      width  = 12
      height = 3
    }
  }

  // PersistGrantsGovXMLDB
  widget {
    group_definition {
      title       = "PersistGrantsGovXMLDB"
      show_title  = true
      layout_type = "ordered"

      widget {
        timeseries_definition {
          title          = "Invocation Status"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            formula {
              formula_expression = "invoke_total - invoke_failure"
              alias              = "Succeeded"
            }
            query {
              metric_query {
                name  = "invoke_total"
                query = "sum:aws.lambda.invocations{$env,$service,$version,functionname:${lower(module.PersistGrantsGovXMLDB.lambda_function_name)}}.as_count()"
              }
            }

            formula {
              formula_expression = "invoke_failure"
              alias              = "Failed"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_failure"
                query = "sum:aws.lambda.errors{$env,$service,$version,functionname:${lower(module.PersistGrantsGovXMLDB.lambda_function_name)}}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 0
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Invocation Duration"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "line"

            formula {
              formula_expression = "invoke_duration"
              alias              = "Duration"
            }
            query {
              metric_query {
                name  = "invoke_duration"
                query = "avg:aws.lambda.duration{$env,$service,$version,functionname:${lower(module.PersistGrantsGovXMLDB.lambda_function_name)}}"
              }
            }

            formula {
              formula_expression = "invoke_timeout"
              alias              = "Timeout"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_timeout"
                query = "avg:aws.lambda.timeout{$env,$service,$version,functionname:${lower(module.PersistGrantsGovXMLDB.lambda_function_name)}}"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 0
          width  = 4
          height = 3
        }
      }

      // Shows counts of opportunities saved vs failed
      widget {
        timeseries_definition {
          title          = "Opportunity Records Saved vs Failed"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          yaxis {
            include_zero = false
            scale        = "sqrt"
          }

          request {
            display_type = "bars"

            formula {
              formula_expression = "records_saved"
              alias              = "Saved"
              style {
                palette       = "cool"
                palette_index = 4
              }
            }
            query {
              metric_query {
                name  = "records_saved"
                query = "sum:grants_ingest.PersistGrantsGovXMLDB.opportunity.saved{$env,$service,$version}.as_count()"
              }
            }

            formula {
              formula_expression = "records_failed"
              alias              = "Failed"
              style {
                palette       = "warm"
                palette_index = 4
              }
            }
            query {
              metric_query {
                name  = "records_failed"
                query = "sum:grants_ingest.PersistGrantsGovXMLDB.opportunity.failed{$env,$service,$version}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 8
          y      = 0
          width  = 4
          height = 3
        }
      }
    }
    widget_layout {
      x      = 0
      y      = 24
      width  = 12
      height = 3
    }
  }

  // PublishGrantEvents
  widget {
    group_definition {
      title       = "PublishGrantEvents"
      show_title  = true
      layout_type = "ordered"

      widget {
        timeseries_definition {
          title          = "Invocation Status"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            formula {
              formula_expression = "invoke_total - invoke_failure"
              alias              = "Succeeded"
            }
            query {
              metric_query {
                name  = "invoke_total"
                query = "sum:aws.lambda.invocations{$env,$service,$version,functionname:${lower(module.PublishGrantEvents.lambda_function_name)}}.as_count()"
              }
            }

            formula {
              formula_expression = "invoke_failure"
              alias              = "Failed"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_failure"
                query = "sum:aws.lambda.errors{$env,$service,$version,functionname:${lower(module.PublishGrantEvents.lambda_function_name)}}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 0
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Invocation Duration"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "line"

            formula {
              formula_expression = "invoke_duration"
              alias              = "Duration"
            }
            query {
              metric_query {
                name  = "invoke_duration"
                query = "avg:aws.lambda.duration{$env,$service,$version,functionname:${lower(module.PublishGrantEvents.lambda_function_name)}}"
              }
            }

            formula {
              formula_expression = "invoke_timeout"
              alias              = "Timeout"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_timeout"
                query = "avg:aws.lambda.timeout{$env,$service,$version,functionname:${lower(module.PublishGrantEvents.lambda_function_name)}}"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 0
          width  = 4
          height = 3
        }
      }

      // Shows counts of events published successfully vs failed to publish
      widget {
        timeseries_definition {
          title          = "Events Published vs Failed"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          yaxis {
            include_zero = false
            scale        = "sqrt"
          }

          request {
            display_type = "bars"

            formula {
              formula_expression = "event_published"
              alias              = "Published"
              style {
                palette       = "cool"
                palette_index = 4
              }
            }
            query {
              metric_query {
                name  = "event_published"
                query = "sum:grants_ingest.PublishGrantEvents.event.published{$env,$service,$version}.as_count()"
              }
            }

            formula {
              formula_expression = "records_failed"
              alias              = "Failed"
              style {
                palette       = "warm"
                palette_index = 4
              }
            }
            query {
              metric_query {
                name  = "records_failed"
                query = "sum:grants_ingest.PublishGrantEvents.event.failed{$env,$service,$version}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 8
          y      = 0
          width  = 4
          height = 3
        }
      }

      // Shows invocation batch sizes from DynamoDB
      widget {
        timeseries_definition {
          title          = "Invocation Batch Size"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          yaxis {
            include_zero = false
            scale        = "sqrt"
          }

          request {
            display_type = "bars"

            formula {
              formula_expression = "batch_size"
              alias              = "Records"
              style {
                palette       = "cool"
                palette_index = 4
              }
            }
            query {
              metric_query {
                name  = "batch_size"
                query = "sum:grants_ingest.PublishGrantEvents.invocation_batch_size{$env,$service,$version}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 0
          y      = 3
          width  = 4
          height = 3
        }
      }

      // Shows DLQ size
      widget {
        timeseries_definition {
          title          = "DLQ Size"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          yaxis {
            include_zero = false
            scale        = "sqrt"
          }

          request {
            display_type = "bars"

            formula {
              formula_expression = "queue_size"
              alias              = "Records"
              style {
                palette       = "warm"
                palette_index = 4
              }
            }
            query {
              metric_query {
                name  = "queue_size"
                query = "sum:aws.sqs.approximate_number_of_messages_visible{$env,$service,$version,queuename:${lower(module.PublishGrantEvents.dlq_name)}}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 3
          width  = 4
          height = 3
        }
      }
    }
    widget_layout {
      x      = 0
      y      = 27
      width  = 12
      height = 6
    }
  }

  // Publishing Operations (from PublishGrantEvents)
  widget {
    group_definition {
      title       = "Publishing Operations"
      show_title  = true
      layout_type = "ordered"

      widget {
        note_definition {
          content          = trimspace(file("${local.datadog_notes_dir}/publishing_operations.md"))
          background_color = "white"
          font_size        = "14"
          has_padding      = true
          text_align       = "left"
          vertical_align   = "top"
          show_tick        = false
        }
        widget_layout {
          x      = 0
          y      = 0
          width  = 4
          height = 13
        }
      }

      widget {
        timeseries_definition {
          title          = "Malformatted Fields"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            formula {
              formula_expression = "occurrences"
            }
            query {
              metric_query {
                name  = "occurrences"
                query = "sum:grants_ingest.PublishGrantEvents.item_image.malformatted_field{$env,$service,$version} by {field}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Build Attempts: New vs Old Images"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            formula {
              formula_expression = "attempts"
            }
            query {
              metric_query {
                name  = "attempts"
                query = "sum:grants_ingest.PublishGrantEvents.item_image.build{$env,$service,$version} by {change}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 8
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Build Attempts: OldImage: Errors"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            style {
              palette = "warm"
            }

            formula {
              alias              = "Images with Invalid Grant Data"
              formula_expression = "invalid"
            }
            query {
              metric_query {
                name  = "invalid"
                query = "sum:grants_ingest.PublishGrantEvents.grant_data.invalid{$env,$service,$version,change:oldimage}.as_count()"
              }
            }

            formula {
              alias              = "Unbuildable Images"
              formula_expression = "unbuildable"
            }
            query {
              metric_query {
                name  = "unbuildable"
                query = "sum:grants_ingest.PublishGrantEvents.item_image.unbuildable{$env,$service,$version,change:oldimage}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 3
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Build Attempts: NewImage: Errors"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            style {
              palette = "warm"
            }

            formula {
              alias              = "Images with Invalid Grant Data"
              formula_expression = "invalid"
            }
            query {
              metric_query {
                name  = "invalid"
                query = "sum:grants_ingest.PublishGrantEvents.grant_data.invalid{$env,$service,$version,change:newimage}.as_count()"
              }
            }

            formula {
              alias              = "Unbuildable Images"
              formula_expression = "unbuildable"
            }
            query {
              metric_query {
                name  = "unbuildable"
                query = "sum:grants_ingest.PublishGrantEvents.item_image.unbuildable{$env,$service,$version,change:newimage}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 8
          y      = 3
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Events Published"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            formula {
              formula_expression = "published"
            }
            query {
              metric_query {
                name  = "published"
                query = "sum:grants_ingest.PublishGrantEvents.event.published{$env,$service,$version}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 6
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Unpublishable Events"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"
            style {
              palette = "warm"
            }

            formula {
              formula_expression = "failures"
            }
            query {
              metric_query {
                name  = "failures"
                query = "sum:grants_ingest.PublishGrantEvents.record.failed{$env,$service,$version}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 8
          y      = 6
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Stream Item Processing Results"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]
          yaxis {
            label = "Items"
            scale = "sqrt"
          }

          request {
            display_type = "bars"

            formula {
              formula_expression = "failed"
              alias              = "Failed"
              style {
                palette       = "warm"
                palette_index = 3
              }
            }
            query {
              metric_query {
                name  = "failed"
                query = "sum:grants_ingest.PublishGrantEvents.record.failed{$env,$service,$version}.as_count()"
              }
            }

            formula {
              formula_expression = "published_by_event_type"
              alias              = "Published"
              style {
                palette       = "green"
                palette_index = 3
              }
            }
            query {
              metric_query {
                name  = "published_by_event_type"
                query = "sum:grants_ingest.PublishGrantEvents.event.published{$env,$service,$version} by {type}.as_count()"
              }
            }

            formula {
              formula_expression = "total_in_invocation - published_total - failed"
              alias              = "Unprocessed"
              style {
                palette       = "gray"
                palette_index = 6
              }
            }
            query {
              metric_query {
                name  = "total_in_invocation"
                query = "sum:grants_ingest.PublishGrantEvents.invocation_batch_size{$env,$service,$version}.as_count()"
              }
            }
            query {
              metric_query {
                name  = "published_total"
                query = "sum:grants_ingest.PublishGrantEvents.event.published{$env,$service,$version}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 9
          width  = 8
          height = 4
        }
      }
    }
    widget_layout {
      x      = 0
      y      = 33
      width  = 12
      height = 13
    }
  }

  // ReceiveFFISEmail
  widget {
    group_definition {
      title       = "ReceiveFFISEmail"
      show_title  = true
      layout_type = "ordered"

      widget {
        timeseries_definition {
          title          = "Invocation Status"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            formula {
              formula_expression = "invoke_total - invoke_failure"
              alias              = "Succeeded"
            }
            query {
              metric_query {
                name  = "invoke_total"
                query = "sum:aws.lambda.invocations{$env,$service,$version,functionname:${lower(module.ReceiveFFISEmail.lambda_function_name)}}.as_count()"
              }
            }

            formula {
              formula_expression = "invoke_failure"
              alias              = "Failed"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_failure"
                query = "sum:aws.lambda.errors{$env,$service,$version,functionname:${lower(module.ReceiveFFISEmail.lambda_function_name)}}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 0
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Invocation Duration"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "line"

            formula {
              formula_expression = "invoke_duration"
              alias              = "Duration"
            }
            query {
              metric_query {
                name  = "invoke_duration"
                query = "avg:aws.lambda.duration{$env,$service,$version,functionname:${lower(module.ReceiveFFISEmail.lambda_function_name)}}"
              }
            }

            formula {
              formula_expression = "invoke_timeout"
              alias              = "Timeout"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_timeout"
                query = "avg:aws.lambda.timeout{$env,$service,$version,functionname:${lower(module.ReceiveFFISEmail.lambda_function_name)}}"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Untrusted Emails"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"
            style {
              palette = "warm"
            }

            formula {
              formula_expression = "untrusted"
            }
            query {
              metric_query {
                name  = "untrusted"
                query = "sum:grants_ingest.ReceiveFFISEmail.email.untrusted{$env,$service,$version}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 8
          y      = 0
          width  = 4
          height = 3
        }
      }
    }
    widget_layout {
      x      = 0
      y      = 46
      width  = 12
      height = 3
    }
  }

  // SplitFFISSpreadsheet
  widget {
    group_definition {
      title       = "SplitFFISSpreadsheet"
      show_title  = true
      layout_type = "ordered"

      widget {
        timeseries_definition {
          title          = "Invocation Status"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            formula {
              formula_expression = "invoke_total - invoke_failure"
              alias              = "Succeeded"
            }
            query {
              metric_query {
                name  = "invoke_total"
                query = "sum:aws.lambda.invocations{$env,$service,$version,functionname:${lower(module.SplitFFISSpreadsheet.lambda_function_name)}}.as_count()"
              }
            }

            formula {
              formula_expression = "invoke_failure"
              alias              = "Failed"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_failure"
                query = "sum:aws.lambda.errors{$env,$service,$version,functionname:${lower(module.SplitFFISSpreadsheet.lambda_function_name)}}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 0
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Invocation Duration"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "line"

            formula {
              formula_expression = "invoke_duration"
              alias              = "Duration"
            }
            query {
              metric_query {
                name  = "invoke_duration"
                query = "avg:aws.lambda.duration{$env,$service,$version,functionname:${lower(module.SplitFFISSpreadsheet.lambda_function_name)}}"
              }
            }

            formula {
              formula_expression = "invoke_timeout"
              alias              = "Timeout"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_timeout"
                query = "avg:aws.lambda.timeout{$env,$service,$version,functionname:${lower(module.SplitFFISSpreadsheet.lambda_function_name)}}"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Grant Opportunities Results"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          yaxis {
            include_zero = false
            scale        = "sqrt"
          }

          request {
            display_type = "bars"

            formula {
              formula_expression = "records_created"
              alias              = "Created"
              style {
                palette = "classic"
              }
            }
            query {
              metric_query {
                name  = "records_created"
                query = "sum:grants_ingest.SplitFFISSpreadsheet.opportunity.created{$env,$service,$version}.as_count()"
              }
            }

            formula {
              formula_expression = "records_failed"
              alias              = "Failed"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "records_failed"
                query = "sum:grants_ingest.SplitFFISSpreadsheet.opportunity.failed{$env,$service,$version}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 8
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Spreadsheet Size"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          yaxis {
            include_zero = false
            scale        = "sqrt"
          }

          request {
            display_type = "bars"

            formula {
              formula_expression = "row_count"
            }
            query {
              metric_query {
                name  = "row_count"
                query = "avg:grants_ingest.SplitFFISSpreadsheet.spreadsheet.row_count{$env,$service,$version}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 0
          y      = 3
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Cell Parsing Errors by Target Field"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            formula {
              formula_expression = "errors"
              style {
                palette = "warm"
              }
            }
            query {
              metric_query {
                name  = "errors"
                query = "sum:grants_ingest.SplitFFISSpreadsheet.spreadsheet.cell_parsing_errors{$env,$service,$version} by {target}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 3
          width  = 4
          height = 3
        }
      }
    }
    widget_layout {
      x      = 0
      y      = 49
      width  = 12
      height = 6
    }
  }

  // SplitGrantsGovXMLDB
  widget {
    group_definition {
      title       = "SplitGrantsGovXMLDB"
      show_title  = true
      layout_type = "ordered"

      widget {
        timeseries_definition {
          title          = "Invocation Status"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "bars"

            formula {
              formula_expression = "invoke_total - invoke_failure"
              alias              = "Succeeded"
            }
            query {
              metric_query {
                name  = "invoke_total"
                query = "sum:aws.lambda.invocations{$env,$service,$version,functionname:${lower(module.SplitGrantsGovXMLDB.lambda_function_name)}}.as_count()"
              }
            }

            formula {
              formula_expression = "invoke_failure"
              alias              = "Failed"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_failure"
                query = "sum:aws.lambda.errors{$env,$service,$version,functionname:${lower(module.SplitGrantsGovXMLDB.lambda_function_name)}}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 0
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Invocation Duration"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          request {
            display_type = "line"

            formula {
              formula_expression = "invoke_duration"
              alias              = "Duration"
            }
            query {
              metric_query {
                name  = "invoke_duration"
                query = "avg:aws.lambda.duration{$env,$service,$version,functionname:${lower(module.SplitGrantsGovXMLDB.lambda_function_name)}}"
              }
            }

            formula {
              formula_expression = "invoke_timeout"
              alias              = "Timeout"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "invoke_timeout"
                query = "avg:aws.lambda.timeout{$env,$service,$version,functionname:${lower(module.SplitGrantsGovXMLDB.lambda_function_name)}}"
              }
            }
          }
        }
        widget_layout {
          x      = 4
          y      = 0
          width  = 4
          height = 3
        }
      }

      widget {
        timeseries_definition {
          title          = "Grant Opportunities Results"
          show_legend    = true
          legend_layout  = "horizontal"
          legend_columns = ["sum"]

          yaxis {
            include_zero = false
            scale        = "sqrt"
          }

          request {
            display_type = "bars"

            formula {
              formula_expression = "opportunities_skipped + records_skipped"
              alias              = "Skipped"
              style {
                palette       = "cool"
                palette_index = 4
              }
            }
            query {
              // Legacy metric (before Forecasted grants were in the pipeline)
              metric_query {
                name  = "opportunities_skipped"
                query = "sum:grants_ingest.SplitGrantsGovXMLDB.opportunity.skipped{$env,$service,$version}.as_count()"
              }
            }
            query {
              metric_query {
                name = "records_skipped"
                query = "sum:grants_ingest.SplitGrantsGovXMLDB.record.skipped{$env,$service,$version}.as_count()"
              }
            }

            formula {
              formula_expression = "opportunities_updated + records_updated"
              alias              = "Updated"
              style {
                palette       = "purple"
                palette_index = 4
              }
            }
            query {
              // Legacy metric (before Forecasted grants were in the pipeline)
              metric_query {
                name  = "opportunities_updated"
                query = "sum:grants_ingest.SplitGrantsGovXMLDB.opportunity.updated{$env,$service,$version}.as_count()"
              }
            }
            query {
              metric_query {
                name  = "records_updated"
                query = "sum:grants_ingest.SplitGrantsGovXMLDB.record.updated{$env,$service,$version}.as_count()"
              }
            }

            formula {
              formula_expression = "opportunities_created + records_created"
              alias              = "Created"
              style {
                palette       = "classic"
                palette_index = 4
              }
            }
            query {
              // Legacy metric (before Forecasted grants were in the pipeline)
              metric_query {
                name  = "opportunities_created"
                query = "sum:grants_ingest.SplitGrantsGovXMLDB.opportunity.created{$env,$service,$version}.as_count()"
              }
            }
            query {
              metric_query {
                name  = "records_created"
                query = "sum:grants_ingest.SplitGrantsGovXMLDB.record.created{$env,$service,$version}.as_count()"
              }
            }

            formula {
              formula_expression = "opportunities_failed + records_failed"
              alias              = "Failed"
              style {
                palette       = "warm"
                palette_index = 5
              }
            }
            query {
              metric_query {
                name  = "opportunities_failed"
                query = "sum:grants_ingest.SplitGrantsGovXMLDB.opportunity.failed{$env,$service,$version}.as_count()"
              }
            }
            query {
              metric_query {
                name  = "records_failed"
                query = "sum:grants_ingest.SplitGrantsGovXMLDB.record.failed{$env,$service,$version}.as_count()"
              }
            }
          }
        }
        widget_layout {
          x      = 8
          y      = 0
          width  = 4
          height = 3
        }
      }
    }
    widget_layout {
      x      = 0
      y      = 55
      width  = 12
      height = 3
    }
  }
}
