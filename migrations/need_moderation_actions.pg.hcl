# Moderation detail payloads referenced from need_progress_events
table "need_moderation_actions" {
  schema = schema.christjesus

  column "id" {
    type = text
  }

  column "need_id" {
    type = text
    null = false
  }

  column "action_type" {
    type    = text
    null    = false
    comment = "review_started, review_note_added, changes_requested, review_approved, review_rejected, document_verified, document_rejected, soft_deleted, restored"
  }

  column "actor_user_id" {
    type    = text
    null    = false
    comment = "Admin user who performed the moderation action"
  }

  column "reason" {
    type    = text
    null    = true
    comment = "Optional reason for approval/rejection/request changes"
  }

  column "note" {
    type    = text
    null    = true
    comment = "Optional free-form moderation note"
  }

  column "document_id" {
    type    = text
    null    = true
    comment = "Optional document reference for document verify/reject actions"
  }

  column "created_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_need_moderation_actions_actor" {
    columns     = [column.actor_user_id]
    ref_columns = [table.users.column.id]
    on_delete   = CASCADE
  }

  foreign_key "fk_need_moderation_actions_need" {
    columns     = [column.need_id]
    ref_columns = [table.needs.column.id]
    on_delete   = CASCADE
  }

  foreign_key "fk_need_moderation_actions_document" {
    columns     = [column.document_id]
    ref_columns = [table.need_documents.column.id]
    on_delete   = SET_NULL
  }

  index "idx_need_moderation_actions_need_created" {
    columns = [column.need_id, column.created_at]
  }

  index "idx_need_moderation_actions_action_type" {
    columns = [column.action_type, column.created_at]
  }

  index "idx_need_moderation_actions_document_id" {
    columns = [column.document_id]
    where   = "document_id IS NOT NULL"
  }
}
