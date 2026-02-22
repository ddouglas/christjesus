
# Need stories
table "need_stories" {
  schema = schema.christjesus

  column "need_id" {
    type = text
    null = false
  }

  column "current" {
    type = text
    null = true
  }

  column "need" {
    type = text
    null = true
  }

  column "outcome" {
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
    columns = [column.need_id]
  }

  foreign_key "fk_need_story_need" {
    columns     = [column.need_id]
    ref_columns = [table.needs.column.id]
    on_delete   = CASCADE
  }

  index "idx_need_stories_need_id" {
    columns = [column.need_id]
  }
}
