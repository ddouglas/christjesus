table "donation_intent_emails" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "donation_intent_id" {
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
    comment = "e.g. receipt, refund_notice — informational, not a constraint"
  }

  column "created_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_donation_intent_emails_intent" {
    columns     = [column.donation_intent_id]
    ref_columns = [table.donation_intents.column.id]
    on_delete   = CASCADE
  }

  foreign_key "fk_donation_intent_emails_message" {
    columns     = [column.email_message_id]
    ref_columns = [table.email_messages.column.id]
    on_delete   = CASCADE
  }

  index "idx_donation_intent_emails_intent_id" {
    columns = [column.donation_intent_id]
  }

  index "idx_donation_intent_emails_message_id" {
    columns = [column.email_message_id]
  }
}
