table "saved_needs" {
  schema = schema.christjesus

  column "user_id" {
    type = text
    null = false
  }

  column "need_id" {
    type = text
    null = false
  }

  column "created_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.user_id, column.need_id]
  }

  foreign_key "fk_saved_needs_user" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_delete   = CASCADE
  }

  foreign_key "fk_saved_needs_need" {
    columns     = [column.need_id]
    ref_columns = [table.needs.column.id]
    on_delete   = CASCADE
  }

  index "idx_saved_needs_user_id" {
    columns = [column.user_id]
  }
}
