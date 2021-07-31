data "frontegg_permission" "read_users" {
  key = "fe.secure.read.users"
}

output "permission_id" {
  value = data.frontegg_permission.read_users.id
}
