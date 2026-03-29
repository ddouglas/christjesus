resource "auth0_tenant" "christjesus" {
  friendly_name = var.workspace != "production" ? "ChristJesus ${title(var.workspace)}" : "ChristJesus"
}

data "auth0_tenant" "current" {}


data "auth0_connection" "users" {
  name = "Username-Password-Authentication"
}

resource "auth0_role" "admin" {
  name        = var.auth0_admin_role_name
  description = "Admin users for ChristJesus app"
}

data "auth0_client" "terraform" {
  name = "Terraform"
}

resource "auth0_client" "web" {
  name            = var.auth0_web_client_name
  app_type        = "regular_web"
  oidc_conformant = true

  grant_types = [
    "authorization_code"
  ]

  jwt_configuration {
    alg                 = "RS256"
    lifetime_in_seconds = 86400
  }

  callbacks           = var.auth0_app_callback_urls
  allowed_logout_urls = var.auth0_app_logout_urls
  web_origins         = var.auth0_app_origins
}

resource "auth0_connection_clients" "users_web" {
  connection_id = data.auth0_connection.users.id
  enabled_clients = [
    auth0_client.web.id,
    data.auth0_client.terraform.id,
  ]
}

resource "auth0_client_credentials" "web" {
  client_id = auth0_client.web.id

  authentication_method = "client_secret_post"
}

resource "auth0_client" "mgmt" {
  name     = "ChristJesus Management"
  app_type = "non_interactive"

  grant_types = ["client_credentials"]
}

resource "auth0_client_credentials" "mgmt" {
  client_id             = auth0_client.mgmt.id
  authentication_method = "client_secret_post"
}

resource "auth0_client_grant" "mgmt_users" {
  client_id = auth0_client.mgmt.id
  audience  = "https://${data.auth0_tenant.current.domain}/api/v2/"
  scopes    = ["update:users", "create:user_tickets"]
}


data "local_file" "inject_user_roles" {
  filename = "${path.module}/actions/inject_user_roles.js"
}

resource "auth0_action" "inject_user_roles" {
  name   = "Inject User Roles"
  code   = data.local_file.inject_user_roles.content
  deploy = true
  supported_triggers {
    id      = "post-login"
    version = "v3"
  }
}

resource "auth0_trigger_actions" "login_flow" {
  trigger = "post-login"

  actions {
    id           = auth0_action.inject_user_roles.id
    display_name = auth0_action.inject_user_roles.name
  }
}

# ---------------------------------------------------------------------------
# Test / service-account users (dev/test only)
# ---------------------------------------------------------------------------

resource "random_string" "test_user_password" {
  length  = 32
  special = false
}

resource "auth0_user" "test_recipient" {
  connection_name = data.auth0_connection.users.name
  email           = "testrecipient@christjesus.app"
  password        = random_string.test_user_password.result
  given_name      = "Test"
  family_name     = "Recipient"

  user_metadata = jsonencode({
    display_name = "Test Recipient"
  })

  depends_on = [
    auth0_connection_clients.users_web
  ]
}

resource "auth0_user" "test_donor" {
  connection_name = data.auth0_connection.users.name
  email           = "testdonor@christjesus.app"
  password        = random_string.test_user_password.result
  given_name      = "Test"
  family_name     = "Donor"

  user_metadata = jsonencode({
    display_name = "Test Donor"
  })

  depends_on = [
    auth0_connection_clients.users_web
  ]
}

resource "auth0_user" "test_admin" {
  connection_name = data.auth0_connection.users.name
  email           = "testadmin@christjesus.app"
  password        = random_string.test_user_password.result
  given_name      = "Test"
  family_name     = "Admin"

  user_metadata = jsonencode({
    display_name = "Test Admin"
  })

  depends_on = [
    auth0_connection_clients.users_web
  ]
}

resource "auth0_user_roles" "test_admin" {
  user_id = auth0_user.test_admin.id
  roles   = [auth0_role.admin.id]
}

resource "local_file" "e2e_test_accounts" {
  filename = "${path.module}/../e2e/test-accounts.json"
  content = jsonencode({
    baseURL  = var.auth0_app_origins[0]
    password = random_string.test_user_password.result
    accounts = {
      recipient = {
        email  = auth0_user.test_recipient.email
        userId = auth0_user.test_recipient.user_id
      }
      donor = {
        email  = auth0_user.test_donor.email
        userId = auth0_user.test_donor.user_id
      }
      admin = {
        email  = auth0_user.test_admin.email
        userId = auth0_user.test_admin.user_id
      }
    }
  })
}
