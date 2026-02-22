env "primary" {
  src = data.hcl_schema.app.url
  url = getenv("DATABASE_URL")

  # Only manage the christjesus schema, ignore all Supabase schemas
  schemas = ["christjesus"]
}

data "hcl_schema" "app" {
  paths = fileset("*.pg.hcl")
}

