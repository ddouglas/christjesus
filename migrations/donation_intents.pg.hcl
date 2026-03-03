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
    type    = uuid
    null    = true
    comment = "Optional authenticated donor user id"
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

  foreign_key "fk_donation_intents_need" {
    columns     = [column.need_id]
    ref_columns = [table.needs.column.id]
    on_delete   = CASCADE
  }

  index "idx_donation_intents_need_created" {
    columns = [column.need_id, column.created_at]
  }

  index "idx_donation_intents_donor_user_id" {
    columns = [column.donor_user_id]
    where   = "donor_user_id IS NOT NULL"
  }
}
