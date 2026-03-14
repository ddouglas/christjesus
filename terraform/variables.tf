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

variable "auth0_app_origin" {
  type        = string
  description = "Allowed web origin for the Auth0 web application"
  default     = "http://localhost:8080"
}

variable "auth0_app_callback_url" {
  type        = string
  description = "Allowed callback URL for the Auth0 web application"
  default     = "http://localhost:8080/auth/callback"
}

variable "auth0_app_logout_url" {
  type        = string
  description = "Allowed logout URL for the Auth0 web application"
  default     = "http://localhost:8080/"
}
