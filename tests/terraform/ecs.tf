resource "aws_ecs_cluster" "main" {
  name = var.project
  tags = {
    Name = var.project
  }
}
