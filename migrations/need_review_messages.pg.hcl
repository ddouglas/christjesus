# User-facing review portal messages between need owners and admins
table "need_review_messages" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "need_id" {
    type = text
    null = false
  }

  column "sender_user_id" {
    type    = text
    null    = false
    comment = "User id of the message author"
  }

  column "sender_role" {
    type    = text
    null    = false
    comment = "user or admin"
  }

  column "body" {
    type = text
    null = false
  }

  column "created_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_need_review_messages_need" {
    columns     = [column.need_id]
    ref_columns = [table.needs.column.id]
    on_delete   = CASCADE
  }

  foreign_key "fk_need_review_messages_sender" {
    columns     = [column.sender_user_id]
    ref_columns = [table.users.column.id]
    on_delete   = CASCADE
  }

  index "idx_need_review_messages_need_created" {
    columns = [column.need_id, column.created_at]
  }

  index "idx_need_review_messages_sender_created" {
    columns = [column.sender_user_id, column.created_at]
  }
}
