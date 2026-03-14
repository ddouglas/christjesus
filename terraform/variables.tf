variable "workspace" {
  type = string
}

variable "cognito_admin_group_name" {
  type        = string
  description = "Cognito group name used for admin authorization"
  default     = "admin"
}
