resource "aws_cognito_user_pool" "user_pool" {
  name = "ChristJesus User Pool ${title(var.workspace)}"

  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }

  email_verification_subject = "Welcome to the Christ Jesus App. Please verify your email address"

  password_policy {
    minimum_length                   = 12
    require_lowercase                = true
    require_numbers                  = true
    require_symbols                  = true
    require_uppercase                = true
    temporary_password_validity_days = 3
  }

  auto_verified_attributes = [
    "email"
  ]

  username_attributes = ["email"]

  user_attribute_update_settings {
    attributes_require_verification_before_update = ["email"]
  }

  deletion_protection = "ACTIVE"

  lifecycle {
    prevent_destroy = true
  }

}

resource "aws_cognito_user_pool_client" "pool_client" {
  user_pool_id = aws_cognito_user_pool.user_pool.id
  name         = "ChristJesus App ${title(var.workspace)}"
  explicit_auth_flows = [
    "ALLOW_USER_PASSWORD_AUTH", "ALLOW_USER_SRP_AUTH", "ALLOW_REFRESH_TOKEN_AUTH"
  ]

  prevent_user_existence_errors = "ENABLED"
  enable_token_revocation       = true

  access_token_validity  = 1
  id_token_validity      = 1
  refresh_token_validity = 30


  token_validity_units {
    access_token  = "hours"
    id_token      = "hours"
    refresh_token = "days"
  }

}

