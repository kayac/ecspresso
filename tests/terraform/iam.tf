resource "aws_iam_role" "ecs-task" {
  name = "${var.project}-ecs-task"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
        Effect = "Allow"
        Sid    = ""
      }
    ]
  })
}

resource "aws_iam_policy" "ecs-task" {
  name = var.project
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "ssm:GetParameter",
          "ssm:GetParameters",
          "secretsmanager:GetSecretValue",
          "ssmmessages:CreateControlChannel",
          "ssmmessages:CreateDataChannel",
          "ssmmessages:OpenControlChannel",
          "ssmmessages:OpenDataChannel",
        ]
        Effect   = "Allow"
        Resource = "*"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "ecs-task" {
  role       = aws_iam_role.ecs-task.name
  policy_arn = aws_iam_policy.ecs-task.arn
}

resource "aws_iam_role" "ecs-task-execution" {
  name = "${var.project}-ecs-task-execution"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
        Effect = "Allow"
        Sid    = ""
      }
    ]
  })
}

data "aws_iam_policy" "ecs-task-exection" {
  arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role_policy_attachment" "ecs-task-execution" {
  role       = aws_iam_role.ecs-task-execution.name
  policy_arn = data.aws_iam_policy.ecs-task-exection.arn
}
