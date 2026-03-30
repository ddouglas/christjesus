table "email_messages" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "recipient" {
    type = text
    null = false
  }

  column "email_type" {
    type    = text
    null    = false
    comment = "e.g. donation_receipt, need_status_update, welcome"
  }

  column "subject" {
    type = text
    null = false
  }

  column "provider" {
    type    = text
    null    = false
    default = "resend"
  }

  column "provider_message_id" {
    type    = text
    null    = true
    comment = "Message ID returned by the provider after successful send"
  }

  column "status" {
    type    = text
    null    = false
    default = "queued"
    comment = "queued, sent, delivered, bounced, complained"
  }

  column "created_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  column "updated_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_email_messages_recipient" {
    columns = [column.recipient]
  }

  index "idx_email_messages_status_created" {
    columns = [column.status, column.created_at]
  }

  index "idx_email_messages_provider_message_id" {
    columns = [column.provider_message_id]
    unique  = true
    where   = "provider_message_id IS NOT NULL"
  }
}
