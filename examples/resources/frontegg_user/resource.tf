resource "frontegg_user" "example" {
  email             = "jane@example.com"
  tenant_id         = "tenant-1234567890"
  skip_invite_email = true

  # Global: the role applies in every environment.
  role_ids = [frontegg_role.viewer.id]

  # Scoped: the role applies only in the given environment.
  environment_role {
    environment_id = "00000000-0000-0000-0000-000000000001"
    role_id        = frontegg_role.admin.id
  }
}
