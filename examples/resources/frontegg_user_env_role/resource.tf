resource "frontegg_user_env_role" "example" {
  tenant_id = "tenant-1234567890"
  user_id   = "user-1234567890"

  # Global: the role applies in every environment.
  assignment {
    role_id = frontegg_role.viewer.id
  }

  # Scoped: the role applies only in the given environment.
  assignment {
    role_id        = frontegg_role.admin.id
    environment_id = "00000000-0000-0000-0000-000000000001"
  }

  assignment {
    role_id        = frontegg_role.admin.id
    environment_id = "00000000-0000-0000-0000-000000000002"
  }
}
