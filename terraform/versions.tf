terraform {
  cloud {

    organization = "christjesus"

    workspaces {
      name = "christjesus-app"
    }
  }
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
    auth0 = {
      source  = "auth0/auth0"
      version = "~> 1.0"
    }
  }
}

provider "aws" {
  region = "us-east-2"
}

provider "auth0" {}
