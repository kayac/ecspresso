resource "aws_cloudwatch_log_group" "main" {
  name              = var.project
  retention_in_days = 7
}
