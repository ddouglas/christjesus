table "users" {
  schema = schema.christjesus

  column "id" {
    type    = uuid
    null    = false
    comment = "Cognito user id (JWT sub)"
  }

  column "user_type" {
    type    = text
    null    = true
    comment = "need, donor, sponsor"
  }

  column "email" {
    type    = text
    null    = true
    comment = "User email from Cognito token"
  }

  column "given_name" {
    type    = text
    null    = true
    comment = "First name from Cognito token"
  }

  column "family_name" {
    type    = text
    null    = true
    comment = "Last name from Cognito token"
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

  index "idx_users_user_type" {
    columns = [column.user_type]
    where   = "user_type IS NOT NULL"
  }

  index "idx_users_email" {
    columns = [column.email]
    where   = "email IS NOT NULL"
  }
}
