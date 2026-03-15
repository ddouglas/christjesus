
# Needs - single table for draft and published
table "needs" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "user_id" {
    type    = text
    null    = false
    comment = "References christjesus.users(id)"
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
    default = "DRAFT"
    comment = "DRAFT, SUBMITTED, READY_FOR_REVIEW, UNDER_REVIEW, CHANGES_REQUESTED, APPROVED, REJECTED, ACTIVE, FUNDED, CLOSED"
  }

  column "verified_at" {
    type = timestamptz
    null = true
  }

  column "verified_by" {
    type    = text
    null    = true
    comment = "Admin user id who verified"
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

  # Soft delete tracking
  column "deleted_at" {
    type    = timestamptz
    null    = true
    comment = "When need was soft deleted by admin"
  }

  column "deleted_by_user_id" {
    type    = text
    null    = true
    comment = "Admin user id that soft deleted this need"
  }

  column "delete_reason" {
    type    = text
    null    = true
    comment = "Required reason captured during admin soft delete"
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

  index "idx_needs_deleted_at" {
    columns = [column.deleted_at]
    where   = "deleted_at IS NOT NULL"
  }

  # Speeds moderation queue pages, which always filter to non-deleted ready/review needs.
  index "idx_needs_queue_active" {
    columns = [column.submitted_at, column.created_at]
    where   = "((deleted_at IS NULL) AND (status = ANY (ARRAY['READY_FOR_REVIEW'::text, 'UNDER_REVIEW'::text])))"
  }

  # Speeds browse/latest lists that only display non-deleted, non-draft needs by recency.
  index "idx_needs_browse_active" {
    columns = [column.created_at]
    where   = "((deleted_at IS NULL) AND (status <> 'DRAFT'::text))"
  }
}