variable "project" {
  type    = string
  default = "ecspresso"
}

provider "aws" {
  region = "ap-northeast-1"
  default_tags {
    tags = {
      "env" = "${var.project}"
    }
  }
}

terraform {
  required_version = ">= 1.4.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 4.65.0"
    }
  }
}

data "aws_caller_identity" "current" {
}

resource "random_string" "random" {
  length  = 8
  lower   = true
  special = false
}
