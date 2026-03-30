table "user_emails" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "user_id" {
    type = text
    null = false
  }

  column "email_message_id" {
    type = text
    null = false
  }

  column "email_type" {
    type    = text
    null    = false
    comment = "e.g. welcome, password_reset, verification — informational, not a constraint"
  }

  column "created_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_user_emails_user" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_delete   = CASCADE
  }

  foreign_key "fk_user_emails_message" {
    columns     = [column.email_message_id]
    ref_columns = [table.email_messages.column.id]
    on_delete   = CASCADE
  }

  index "idx_user_emails_user_id" {
    columns = [column.user_id]
  }

  index "idx_user_emails_message_id" {
    columns = [column.email_message_id]
  }
}
