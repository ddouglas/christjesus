table "zip_centroids" {
  schema = schema.christjesus

  column "zip_code" {
    type    = text
    null    = false
    comment = "5-digit US ZIP code"
  }

  column "latitude" {
    type = decimal(9, 6)
    null = false
  }

  column "longitude" {
    type = decimal(9, 6)
    null = false
  }

  primary_key {
    columns = [column.zip_code]
  }
}
