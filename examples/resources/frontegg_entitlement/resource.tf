resource "frontegg_entitlement" "example" {
  # Tenant-level grant
  entitlement {
    plan_id   = "00000000-0000-0000-0000-000000000001"
    tenant_id = "11111111-1111-1111-1111-111111111111"
  }

  # Tenant-level grant with expiration
  entitlement {
    plan_id         = "00000000-0000-0000-0000-000000000002"
    tenant_id       = "11111111-1111-1111-1111-111111111111"
    expiration_date = "2030-01-01T00:00:00Z"
  }

  # User-level grant within the same tenant.
  # Tenant-level and user-level entitlements for the same (plan, tenant) may coexist.
  entitlement {
    plan_id   = "00000000-0000-0000-0000-000000000001"
    tenant_id = "11111111-1111-1111-1111-111111111111"
    user_id   = "22222222-2222-2222-2222-222222222222"
  }
}
