resource "tigris_bucket" "documents" {
  bucket               = "christjesus-documents-${var.workspace}"
  default_storage_tier = "STANDARD"
}
