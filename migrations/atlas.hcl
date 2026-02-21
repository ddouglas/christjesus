env "local" {
  src = data.hcl_schema.app.url
  url = getenv("DATABASE_URL")
}

env "supabase" {
  src = data.hcl_schema.app.url
  url = getenv("SUPABASE_DB_URL")
}

data "hcl_schema" "app" {
  paths = fileset("*.pg.hcl")
}
