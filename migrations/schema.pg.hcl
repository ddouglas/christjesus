schema "christjesus" {}
table "need_categories" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "name" {
    type = text
    null = false
  }

  column "slug" {
    type = text
    null = false
  }

  column "description" {
    type = text
    null = true
  }

  column "icon" {
    type    = text
    null    = true
    comment = "Icon identifier for UI"
  }

  column "display_order" {
    type    = integer
    null    = false
    default = 0
  }

  column "is_active" {
    type    = boolean
    null    = false
    default = true
  }

  column "created_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  unique "uq_need_categories_slug" {
    columns = [column.slug]
  }

  index "idx_need_categories_is_active" {
    columns = [column.is_active]
  }
}

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

  # Location (Step 2)
  column "address" {
    type    = text
    null    = true
    comment = "Street address. Required when status = submitted"
  }

  column "address_ext" {
    type    = text
    null    = true
    comment = "Apartment, suite, unit number (optional)"
  }

  column "city" {
    type    = text
    null    = true
    comment = "Required when status = submitted"
  }

  column "state" {
    type    = text
    null    = true
    comment = "Required when status = submitted"
  }

  column "zip_code" {
    type    = text
    null    = true
    comment = "Required when status = submitted"
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

  # Details (Step 4)
  column "title" {
    type    = text
    null    = true
    comment = "Required when status = submitted"
  }

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

  # Story (Step 5)
  column "story" {
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
    comment = "Tracks onboarding progress: welcome, location, categories, details, story, documents, review"
  }

  column "completed_steps" {
    type    = jsonb
    null    = false
    default = "[]"
    comment = "Array of completed step names"
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

  index "idx_needs_user_id" {
    columns = [column.user_id]
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

  index "idx_needs_city_state" {
    columns = [column.city, column.state]
    where   = "status IN ('active', 'funded')"
  }

  index "idx_needs_user_status" {
    columns = [column.user_id, column.status]
  }
}

# Need category assignments
table "need_category_assignments" {
  schema = schema.christjesus

  column "need_id" {
    type = text
    null = false
  }

  column "category_id" {
    type = text
    null = false
  }

  column "created_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.need_id, column.category_id]
  }

  foreign_key "fk_need_categories_need" {
    columns     = [column.need_id]
    ref_columns = [table.needs.column.id]
    on_delete   = CASCADE
  }

  foreign_key "fk_need_categories_category" {
    columns     = [column.category_id]
    ref_columns = [table.need_categories.column.id]
    on_delete   = CASCADE
  }

  index "idx_need_assignments_need_id" {
    columns = [column.need_id]
  }

  index "idx_need_assignments_category_id" {
    columns = [column.category_id]
  }
}

# Document uploads
table "need_documents" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "need_id" {
    type = text
    null = false
  }

  column "user_id" {
    type    = uuid
    null    = false
    comment = "References auth.users(id) - no FK"
  }

  column "document_type" {
    type    = text
    null    = false
    comment = "id, utility_bill, medical_record, income_verification, etc."
  }

  column "file_name" {
    type = text
    null = false
  }

  column "file_size_bytes" {
    type = integer
    null = false
  }

  column "mime_type" {
    type = text
    null = false
  }

  column "storage_key" {
    type    = text
    null    = false
    comment = "Supabase storage bucket key/path"
  }

  column "uploaded_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_documents_need" {
    columns     = [column.need_id]
    ref_columns = [table.needs.column.id]
    on_delete   = CASCADE
  }

  index "idx_documents_need_id" {
    columns = [column.need_id]
  }

  index "idx_documents_user_id" {
    columns = [column.user_id]
  }
}