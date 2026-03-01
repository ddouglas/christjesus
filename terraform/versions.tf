terraform {
  backend "s3" {
    region       = "us-east-2"
    bucket       = "cja-us-east-2-863sxr"
    key          = "app/terraform.tfstate"
    use_lockfile = true
    encrypt      = true
  }
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

provider "aws" {
  region = "us-east-2"
}

