table "users" {
  schema = schema.christjesus

  column "id" {
    type    = text
    null    = false
    comment = "Internal application user identifier (NanoID)"
  }

  column "auth_subject" {
    type    = text
    null    = true
    comment = "External auth provider subject claim (for example Auth0 sub)"
  }

  column "user_type" {
    type    = text
    null    = true
    comment = "recipient, donor, sponsor"
  }

  column "email" {
    type    = text
    null    = true
    comment = "User email from ID token"
  }

  column "given_name" {
    type    = text
    null    = true
    comment = "First name from ID token"
  }

  column "family_name" {
    type    = text
    null    = true
    comment = "Last name from ID token"
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

  index "idx_users_auth_subject" {
    columns = [column.auth_subject]
    unique  = true
  }
}
