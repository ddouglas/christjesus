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
  value = data.auth0_connection.users.id
}

output "auth0_db_connection_name" {
  value = data.auth0_connection.users.id
}

output "auth0_admin_role_name" {
  value = auth0_role.admin.name
}

output "auth0_mgmt_client_id" {
  value = auth0_client.mgmt.client_id
}

output "auth0_mgmt_client_secret" {
  value     = auth0_client_credentials.mgmt.client_secret
  sensitive = true
}

# output "test_user_password" {
#   value = random_string.test_user_password.result
# }

# output "test_recipient_email" {
#   value = auth0_user.test_recipient.email
# }

# output "test_recipient_user_id" {
#   value = auth0_user.test_recipient.user_id
# }

# output "test_donor_email" {
#   value = auth0_user.test_donor.email
# }

# output "test_donor_user_id" {
#   value = auth0_user.test_donor.user_id
# }

# output "test_admin_email" {
#   value = auth0_user.test_admin.email
# }

# output "test_admin_user_id" {
#   value = auth0_user.test_admin.user_id
# }
