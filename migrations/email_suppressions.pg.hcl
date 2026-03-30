table "email_suppressions" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "email_address" {
    type = text
    null = false
  }

  column "reason" {
    type    = text
    null    = false
    comment = "hard_bounce, complaint, manual"
  }

  column "source_event_id" {
    type    = text
    null    = true
    comment = "FK to email_events that triggered this suppression"
  }

  column "created_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  column "removed_at" {
    type    = timestamptz
    null    = true
    comment = "Null if still active; set when suppression is lifted"
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_email_suppressions_event" {
    columns     = [column.source_event_id]
    ref_columns = [table.email_events.column.id]
    on_delete   = SET_NULL
  }

  index "idx_email_suppressions_address" {
    columns = [column.email_address]
    unique  = true
    where   = "removed_at IS NULL"
  }
}
