table "donor_preference_category_assignments" {
  schema = schema.christjesus

  column "user_id" {
    type    = uuid
    null    = false
    comment = "References auth.users(id) from Supabase/Cognito-linked users - no FK due to different schema"
  }

  column "category_id" {
    type = text
    null = false
  }

  column "created_at" {
    type = timestamptz
    null = false
  }

  primary_key {
    columns = [column.user_id, column.category_id]
  }

  foreign_key "fk_donor_pref_category_assignments_category" {
    columns     = [column.category_id]
    ref_columns = [table.need_categories.column.id]
    on_delete   = CASCADE
  }

  index "idx_donor_pref_assignments_user_id" {
    columns = [column.user_id]
  }

  index "idx_donor_pref_assignments_category_id" {
    columns = [column.category_id]
  }
}
