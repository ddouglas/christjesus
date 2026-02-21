schema "public" {}

table "prayer_requests" {
  schema = schema.public

  column "id" {
    type = text
  }

  column "name" {
    type = text
    null = false
  }

  column "email" {
    type = text
    null = true
  }

  column "request_body" {
    type = text
    null = false
  }

  column "created_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_prayer_requests_created_at" {
    columns = [column.created_at]
  }
}

table "email_signups" {
  schema = schema.public

  column "email" {
    type = text
    null = false
  }

  column "city" {
    type = text
    null = true
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
    columns = [column.email]
  }
}
