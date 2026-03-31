locals {
  cloudflare_account_id = data.sops_file.terraform.data["cloudflare_account_id"]
}


variable "workspace" {
  type = string
}

variable "auth0_admin_role_name" {
  type        = string
  description = "Auth0 role name used for admin authorization"
  default     = "admin"
}

variable "auth0_web_client_name" {
  type        = string
  description = "Name of the Auth0 regular web application client"
  default     = "ChristJesus Web"
}

variable "auth0_db_connection_name" {
  type        = string
  description = "Name of the Auth0 database connection for native user management"
  default     = "christjesus"
}

variable "auth0_app_origins" {
  type        = list(string)
  description = "Allowed web origins for the Auth0 web application"
  default     = ["http://localhost:8080"]
}

variable "auth0_app_callback_urls" {
  type        = list(string)
  description = "Allowed callback URLs for the Auth0 web application"
  default     = ["http://localhost:8080/auth/callback"]
}

variable "auth0_app_logout_urls" {
  type        = list(string)
  description = "Allowed logout URLs for the Auth0 web application"
  default     = ["http://localhost:8080/"]
}

variable "cloudflare_account_id" {
  type = string
}
