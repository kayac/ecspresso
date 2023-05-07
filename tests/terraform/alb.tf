resource "aws_lb" "main" {
  name               = var.project
  internal           = false
  load_balancer_type = "application"
  security_groups = [
    aws_security_group.alb.id,
    aws_security_group.default.id,
  ]
  subnets = [
    aws_subnet.public-a.id,
    aws_subnet.public-c.id,
    aws_subnet.public-d.id,
  ]
  tags = {
    Name = var.project
  }
}

resource "aws_lb_target_group" "http" {
  name                 = "${var.project}-http"
  port                 = 80
  target_type          = "ip"
  vpc_id               = aws_vpc.main.id
  protocol             = "HTTP"
  deregistration_delay = 5

  health_check {
    path                = "/"
    port                = "traffic-port"
    protocol            = "HTTP"
    healthy_threshold   = 2
    unhealthy_threshold = 10
    timeout             = 5
    interval            = 6
  }
  tags = {
    Name = "${var.project}-http"
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.http.arn
  }

  tags = {
    Name = "${var.project}-http"
  }
}

output "alb_dns_name" {
  value = aws_lb.main.dns_name
}
