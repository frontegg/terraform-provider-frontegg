resource "frontegg_portal_user" "example" {
  email     = "portal-user@example.com"
  password  = "SecurePassword123!"
  tenant_id = "your-tenant-id"
  role_ids = [
    "role-id-1",
    "role-id-2",
  ]
}

