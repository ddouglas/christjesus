table "stripe_webhook_events" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "stripe_event_id" {
    type = text
    null = false
  }

  column "event_type" {
    type = text
    null = false
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

  index "idx_stripe_webhook_events_stripe_event_id" {
    columns = [column.stripe_event_id]
    unique  = true
  }
}