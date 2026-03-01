resource "aws_s3_bucket" "documents" {
  bucket = "cja-${var.workspace}-${local.region}"
}

resource "aws_s3_bucket_versioning" "documents" {
  bucket = aws_s3_bucket.documents.bucket
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "documents" {
  bucket = aws_s3_bucket_versioning.documents.bucket
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_acl" "documents" {
  depends_on = [aws_s3_bucket_ownership_controls.documents]
  bucket     = aws_s3_bucket.documents.bucket
  acl        = "private"
}

resource "aws_s3_bucket_ownership_controls" "documents" {
  bucket = aws_s3_bucket.documents.bucket
  rule {
    object_ownership = "BucketOwnerPreferred"
  }
}
