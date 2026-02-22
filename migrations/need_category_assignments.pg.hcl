
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