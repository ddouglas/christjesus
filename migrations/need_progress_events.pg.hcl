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
    comment = "Timeline event key (onboarding + moderation actions)"
  }

  column "event_source" {
    type    = text
    null    = false
    default = "user"
    comment = "user, admin, system"
  }

  column "actor_user_id" {
    type    = uuid
    null    = true
    comment = "Actor user id when event is user/admin initiated"
  }

  column "moderation_action_id" {
    type    = text
    null    = true
    comment = "Reference to christjesus.need_moderation_actions.id for moderation detail content"
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

  foreign_key "fk_progress_moderation_action" {
    columns     = [column.moderation_action_id]
    ref_columns = [table.need_moderation_actions.column.id]
    on_delete   = SET_NULL
  }

  index "idx_progress_need_id" {
    columns = [column.need_id]
  }

  index "idx_progress_need_created" {
    columns = [column.need_id, column.created_at]
  }

  index "idx_progress_step_created" {
    columns = [column.step, column.created_at]
  }

  index "idx_progress_moderation_action_id" {
    columns = [column.moderation_action_id]
    where   = "moderation_action_id IS NOT NULL"
  }
}