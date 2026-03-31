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
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
    render = {
      source  = "render-oss/render"
      version = "~> 1.0"
    }
    sops = {
      source  = "carlpett/sops"
      version = "~> 1.0"
    }
    tigris = {
      source  = "tigrisdata/tigris"
      version = "~> 1.0"
    }
  }
}

provider "sops" {}

provider "auth0" {
  domain        = var.auth0_domain
  client_id     = var.auth0_client_id
  client_secret = data.sops_file.terraform.data["auth0_client_secret"]
}

provider "tigris" {
  access_key = data.sops_file.terraform.data["tigris_access_key"]
  secret_key = data.sops_file.terraform.data["tigris_secret_key"]
}

provider "render" {
  api_key                    = data.sops_file.terraform.data["render_api_key"]
  owner_id                   = var.render_owner_id
  wait_for_deploy_completion = true
}

provider "cloudflare" {
  api_token = data.sops_file.terraform.data["cloudflare_api_token"]
}
