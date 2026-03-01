table "user_addresses" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "user_id" {
    type    = uuid
    null    = false
    comment = "References auth.users(id) from Supabase - no FK due to different schema"
  }

  column "address" {
    type    = text
    null    = true
    comment = "Street address"
  }

  column "address_ext" {
    type    = text
    null    = true
    comment = "Apartment, suite, unit number (optional)"
  }

  column "city" {
    type = text
    null = true
  }

  column "state" {
    type = text
    null = true
  }

  column "zip_code" {
    type = text
    null = true
  }

  column "privacy_display" {
    type    = text
    null    = true
    default = "neighborhood"
    comment = "What to show publicly: neighborhood, zip, or city"
  }

  column "contact_methods" {
    type    = jsonb
    null    = true
    comment = "Array of preferred contact methods: phone, text, email"
  }

  column "preferred_contact_time" {
    type    = text
    null    = true
    comment = "Preferred time to contact: morning, afternoon, evening, anytime"
  }

  column "is_primary" {
    type    = boolean
    null    = false
    default = false
    comment = "One primary address per user"
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

  index "idx_user_addresses_user_id" {
    columns = [column.user_id]
  }

  index "idx_user_addresses_primary_by_user" {
    columns = [column.user_id]
    unique  = true
    where   = "is_primary = true"
  }
}
