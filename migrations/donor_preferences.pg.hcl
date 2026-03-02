table "donor_preferences" {
  schema = schema.christjesus

  column "user_id" {
    type    = uuid
    null    = false
    comment = "References auth.users(id) from Supabase/Cognito-linked users - no FK due to different schema"
  }

  column "zip_code" {
    type = text
    null = true
  }

  column "radius" {
    type    = text
    null    = true
    comment = "5-miles, 15-miles, 25-miles, anywhere"
  }

  column "donation_range" {
    type    = text
    null    = true
    comment = "under-50, 50-100, 100-500, 500-plus"
  }

  column "notification_frequency" {
    type    = text
    null    = true
    comment = "daily, weekly, monthly, never"
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
    columns = [column.user_id]
  }
}
