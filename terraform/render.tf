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
