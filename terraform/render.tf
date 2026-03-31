resource "render_project" "web" {
  name = "bodyofchrist-web"
  environments = {
    "development" = {
      name             = "development"
      protected_status = "unprotected"
      network_isolated = true
    }
  }
}

resource "render_registry_credential" "github" {
  auth_token = data.sops_file.terraform.data["github_personal_access_token"]
  name       = "github"
  registry   = "GITHUB"
  username   = "ddouglas"
}

resource "render_web_service" "app" {
  name   = "bodyofchrist-development"
  plan   = "starter"
  region = "virginia"


  runtime_source = {
    image = {
      image_url               = "ghcr.io/ddouglas/christjesus"
      registray_credential_id = render_registry_credential.github.id
      tag                     = "latest"
    }
  }

  env_vars = {
    for k, v in data.sops_file.app.data : k => { value = v }
  }

  custom_domains = [
    { name = "development.bodyofchrist.app" }
  ]
}
