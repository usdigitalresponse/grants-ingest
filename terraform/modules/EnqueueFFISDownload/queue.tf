

resource "aws_sqs_queue" "ffis_downloads" {
  name = "ffis_downloads"

  delay_seconds              = 0
  visibility_timeout_seconds = 15 * 60
  receive_wait_time_seconds  = 20
  message_retention_seconds  = 5 * 60 * 60 * 24 # 5 days
  max_message_size           = 1024             # 1 KB
}
