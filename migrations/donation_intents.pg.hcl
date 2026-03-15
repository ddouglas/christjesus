table "donation_intents" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "need_id" {
    type = text
    null = false
  }

  column "donor_user_id" {
    type    = text
    null    = true
    comment = "Optional authenticated donor user id"
  }

  column "checkout_session_id" {
    type    = text
    null    = true
    comment = "Stripe checkout session id"
  }

  column "payment_intent_id" {
    type    = text
    null    = true
    comment = "Stripe payment intent id"
  }

  column "amount_cents" {
    type = integer
    null = false
  }

  column "private_message" {
    type = text
    null = true
  }

  column "is_anonymous" {
    type    = boolean
    null    = false
    default = false
  }

  column "payment_provider" {
    type    = text
    null    = false
    default = "stripe"
  }

  column "payment_status" {
    type    = text
    null    = false
    default = "pending"
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

  foreign_key "fk_donation_intents_donor" {
    columns     = [column.donor_user_id]
    ref_columns = [table.users.column.id]
    on_delete   = SET_NULL
  }

  foreign_key "fk_donation_intents_need" {
    columns     = [column.need_id]
    ref_columns = [table.needs.column.id]
    on_delete   = CASCADE
  }

  index "idx_donation_intents_need_created" {
    columns = [column.need_id, column.created_at]
  }

  index "idx_donation_intents_checkout_session_id" {
    columns = [column.checkout_session_id]
    where   = "checkout_session_id IS NOT NULL"
  }

  index "idx_donation_intents_donor_user_id" {
    columns = [column.donor_user_id]
    where   = "donor_user_id IS NOT NULL"
  }

  index "idx_donation_intents_payment_intent_updated" {
    columns = [column.payment_intent_id, column.updated_at]
    where   = "payment_intent_id IS NOT NULL"
  }

  index "idx_donation_intents_pending_created_at" {
    columns = [column.created_at]
    where   = "(payment_status = 'pending'::text)"
  }
}
