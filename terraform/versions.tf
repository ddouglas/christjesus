terraform {
  cloud {
    organization = "christjesus"
    workspaces {
      name = "christjesus-app"
    }
  }
  required_providers {
    auth0 = {
      source  = "auth0/auth0"
      version = "~> 1.0"
    }
    tigris = {
      source  = "tigrisdata/tigris"
      version = "~> 1.0"
    }
  }
}

provider "auth0" {}

provider "tigris" {
  access_key = var.tigris_access_key
  secret_key = var.tigris_secret_key
}

