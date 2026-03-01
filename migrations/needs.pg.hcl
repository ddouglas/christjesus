
# Needs - single table for draft and published
table "needs" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "user_id" {
    type    = uuid
    null    = false
    comment = "References auth.users(id) from Supabase - no FK due to different schema"
  }

  column "user_address_id" {
    type    = text
    null    = true
    comment = "References christjesus.user_addresses(id) for selected address on this need"
  }

  column "uses_non_primary_address" {
    type    = boolean
    null    = false
    default = false
    comment = "True when need references a non-primary user address"
  }

  # Details (Step 4)
  # column "title" {
  #   type    = text
  #   null    = true
  #   comment = "Required when status = submitted"
  # }

  column "amount_needed_cents" {
    type    = integer
    null    = false
    comment = "Amount in cents. Required when status = submitted"
    default = 0
  }

  column "amount_raised_cents" {
    type    = integer
    null    = false
    default = 0
  }

  column "short_description" {
    type    = text
    null    = true
    comment = "Required when status = submitted"
  }

  # Status tracking
  column "status" {
    type    = text
    null    = false
    default = "draft"
    comment = "draft, submitted, pending_review, approved, rejected, active, funded, closed"
  }

  column "verified_at" {
    type = timestamptz
    null = true
  }

  column "verified_by" {
    type    = uuid
    null    = true
    comment = "Admin user who verified - no FK"
  }

  # Progress tracking (for draft status)
  column "current_step" {
    type    = text
    null    = false
    default = "welcome"
    comment = "Tracks onboarding progress: welcome, location, categories, story, documents, review"
  }

  # Visibility
  column "published_at" {
    type    = timestamptz
    null    = true
    comment = "When status changed to active/published"
  }

  column "closed_at" {
    type = timestamptz
    null = true
  }

  column "is_featured" {
    type    = boolean
    null    = false
    default = false
  }

  # Submission tracking
  column "submitted_at" {
    type    = timestamptz
    null    = true
    comment = "When user completed onboarding and submitted"
  }

  # Metadata
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

  foreign_key "fk_needs_user_address" {
    columns     = [column.user_address_id]
    ref_columns = [table.user_addresses.column.id]
    on_delete   = SET_NULL
  }

  index "idx_needs_user_id" {
    columns = [column.user_id]
  }

  index "idx_needs_user_address_id" {
    columns = [column.user_address_id]
    where   = "user_address_id IS NOT NULL"
  }

  index "idx_needs_status" {
    columns = [column.status]
  }

  index "idx_needs_published_at" {
    columns = [column.published_at]
    where   = "published_at IS NOT NULL"
  }

  index "idx_needs_is_featured" {
    columns = [column.is_featured]
    where   = "is_featured = true"
  }

  index "idx_needs_user_status" {
    columns = [column.user_id, column.status]
  }
}