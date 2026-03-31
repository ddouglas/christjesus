data "sops_file" "terraform" {
  source_file = "${path.module}/configs/${var.workspace}/terraform.enc.yaml"
  input_type  = "yaml"
}

data "sops_file" "app" {
  source_file = "${path.module}/../configs/${var.workspace}/app.enc.yaml"
  input_type  = "yaml"
}
