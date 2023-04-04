locals {
  datadog_draft_label = var.datadog_draft ? "(Draft - ${var.environment})" : ""
}

resource "datadog_dashboard" "service_dashboard" {
  title       = trimspace("Grants Ingest Service Dashboard ${local.datadog_draft_label}")
  description = "Dashboard for monitoring the Grants Ingest pipeline service."
  layout_type = "ordered"
  reflow_type = "auto"

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
              formula_expression = "invoke_success"
              alias              = "Succeeded"
            }
            query {
              metric_query {
                name  = "invoke_success"
                query = "sum:aws.lambda.invocations{$env,$service,$version,handlername:downloadgrantsgovdb}.as_count()"
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
                query = "sum:aws.lambda.errors{$env,$service,$version,handlername:downloadgrantsgovdb}.as_count()"
              }
            }
          }
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
                query = "avg:aws.lambda.duration{$env,$service,$version,handlername:downloadgrantsgovdb}"
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
                query = "avg:aws.lambda.timeout{$env,$service,$version,handlername:downloadgrantsgovdb}"
              }
            }
          }
        }
      }

      widget {
        timeseries_definition {
          title          = "Source File Size"
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
                query = "avg:grants_ingest.DownloadGrantsGovDB.source_size{$env,$service,$version,handlername:downloadgrantsgovdb}"
              }
            }
          }
        }
      }
    }
  }

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
              formula_expression = "invoke_success"
              alias              = "Succeeded"
            }
            query {
              metric_query {
                name  = "invoke_success"
                query = "sum:aws.lambda.invocations{$env,$service,$version,handlername:splitgrantsgovxmldb}.as_count()"
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
                query = "sum:aws.lambda.errors{$env,$service,$version,handlername:splitgrantsgovxmldb}.as_count()"
              }
            }
          }
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
                query = "avg:aws.lambda.duration{$env,$service,$version,handlername:splitgrantsgovxmldb}"
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
                query = "avg:aws.lambda.timeout{$env,$service,$version,handlername:splitgrantsgovxmldb}"
              }
            }
          }
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
              formula_expression = "records_skipped"
              alias              = "Skipped"
              style {
                palette       = "cool"
                palette_index = 4
              }
            }
            query {
              metric_query {
                name  = "records_skipped"
                query = "sum:grants_ingest.SplitGrantsGovXMLDB.opportunity.skipped{$env,$service,$version,handlername:splitgrantsgovxmldb}.as_count()"
              }
            }

            formula {
              formula_expression = "records_updated"
              alias              = "Updated"
              style {
                palette       = "purple"
                palette_index = 4
              }
            }
            query {
              metric_query {
                name  = "records_updated"
                query = "sum:grants_ingest.SplitGrantsGovXMLDB.opportunity.updated{$env,$service,$version,handlername:splitgrantsgovxmldb}.as_count()"
              }
            }

            formula {
              formula_expression = "records_created"
              alias              = "Created"
              style {
                palette       = "classic"
                palette_index = 4
              }
            }
            query {
              metric_query {
                name  = "records_created"
                query = "sum:grants_ingest.SplitGrantsGovXMLDB.opportunity.created{$env,$service,$version,handlername:splitgrantsgovxmldb}.as_count()"
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
                query = "sum:grants_ingest.SplitGrantsGovXMLDB.opportunity.failed{$env,$service,$version,handlername:splitgrantsgovxmldb}.as_count()"
              }
            }
          }
        }
      }
    }
  }
}
