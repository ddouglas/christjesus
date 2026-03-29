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

  column "geog" {
    type    = sql("christjesus.geography(Point, 4326)")
    null    = true
    comment = "PostGIS geography point derived from latitude/longitude"
  }

  primary_key {
    columns = [column.zip_code]
  }

  index "idx_zip_centroids_geog" {
    columns = [column.geog]
    type    = GiST
  }
}
