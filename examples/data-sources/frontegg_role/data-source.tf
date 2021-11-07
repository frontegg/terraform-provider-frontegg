data "frontegg_role" "admin" {
  key = "Admin"
}

output "role_id" {
  value = data.frontegg_role.admin.id
}
