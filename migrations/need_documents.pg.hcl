
# Document uploads
table "need_documents" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "need_id" {
    type = text
    null = false
  }

  column "user_id" {
    type    = uuid
    null    = false
    comment = "References auth.users(id) - no FK"
  }

  column "document_type" {
    type    = text
    null    = false
    comment = "id, utility_bill, medical_record, income_verification, etc."
  }

  column "file_name" {
    type = text
    null = false
  }

  column "file_size_bytes" {
    type = integer
    null = false
  }

  column "mime_type" {
    type = text
    null = false
  }

  column "storage_key" {
    type    = text
    null    = false
    comment = "Supabase storage bucket key/path"
  }

  column "uploaded_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_documents_need" {
    columns     = [column.need_id]
    ref_columns = [table.needs.column.id]
    on_delete   = CASCADE
  }

  index "idx_documents_need_id" {
    columns = [column.need_id]
  }

  index "idx_documents_user_id" {
    columns = [column.user_id]
  }
}