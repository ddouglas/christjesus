# Progress tracking events for analytics
table "need_progress_events" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "need_id" {
    type = text
    null = false
  }

  column "step" {
    type    = text
    null    = false
    comment = "Step completed: welcome, location, categories, details, story, documents, review, submitted"
  }

  column "created_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_progress_need" {
    columns     = [column.need_id]
    ref_columns = [table.needs.column.id]
    on_delete   = CASCADE
  }

  index "idx_progress_need_id" {
    columns = [column.need_id]
  }

  index "idx_progress_step_created" {
    columns = [column.step, column.created_at]
  }
}