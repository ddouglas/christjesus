resource "auth0_tenant" "christjesus" {
  friendly_name = var.workspace != "production" ? "ChristJesus ${title(var.workspace)}" : "ChristJesus"
}

resource "auth0_connection" "users" {
  name     = "${var.auth0_db_connection_name}-${var.workspace}"
  strategy = "auth0"
}

resource "auth0_role" "admin" {
  name        = var.auth0_admin_role_name
  description = "Admin users for ChristJesus app"
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

  callbacks = [
    var.auth0_app_callback_url,
  ]

  allowed_logout_urls = [
    var.auth0_app_logout_url,
  ]

  web_origins = [
    var.auth0_app_origin,
  ]
}

resource "auth0_connection_clients" "users_web" {
  connection_id = auth0_connection.users.id
  enabled_clients = [
    auth0_client.web.id,
  ]
}

resource "auth0_client_credentials" "web" {
  client_id = auth0_client.web.id

  authentication_method = "client_secret_post"
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
