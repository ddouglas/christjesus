table "email_events" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "email_message_id" {
    type    = text
    null    = true
    comment = "Nullable — may not resolve if provider_message_id is unknown"
  }

  column "provider_event_id" {
    type    = text
    null    = false
    comment = "Idempotency key from the provider"
  }

  column "event_type" {
    type    = text
    null    = false
    comment = "e.g. delivered, bounced, complained, opened, clicked"
  }

  column "payload" {
    type = jsonb
    null = true
  }

  column "created_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_email_events_message" {
    columns     = [column.email_message_id]
    ref_columns = [table.email_messages.column.id]
    on_delete   = SET_NULL
  }

  index "idx_email_events_provider_event_id" {
    columns = [column.provider_event_id]
    unique  = true
  }

  index "idx_email_events_message_id" {
    columns = [column.email_message_id]
    where   = "email_message_id IS NOT NULL"
  }
}
