output "documents_bucket" {
  value = tigris_bucket.documents.bucket
}

output "auth0_web_client_id" {
  value = auth0_client.web.client_id
}

output "auth0_web_client_name" {
  value = auth0_client.web.name
}

output "auth0_db_connection_id" {
  value = auth0_connection.users.id
}

output "auth0_db_connection_name" {
  value = auth0_connection.users.name
}

output "auth0_admin_role_name" {
  value = auth0_role.admin.name
}
