
resource "aws_ssm_parameter" "foo" {
  name  = "/${var.project}/foo-${random_string.random.result}"
  type  = "SecureString"
  value = "FOO"
}

resource "aws_secretsmanager_secret" "bar" {
  name = "${var.project}-bar-${random_string.random.result}"
}

resource "aws_secretsmanager_secret_version" "bar" {
  secret_id     = aws_secretsmanager_secret.bar.id
  secret_string = "BAR"
}

resource "aws_secretsmanager_secret" "json" {
  name = "${var.project}-json-${random_string.random.result}"
}

resource "aws_secretsmanager_secret_version" "json" {
  secret_id = aws_secretsmanager_secret.json.id
  secret_string = jsonencode({
    key = "value"
  })
}
