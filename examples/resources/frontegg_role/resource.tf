resource "frontegg_role" "example" {
  name        = "Example"
  key         = "example"
  description = "An example of a role"
  level       = 0
  permission_ids = [
    resource.frontegg_permission.example.id,
    data.frontegg_permission.read_users.id,
  ]
}
