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
