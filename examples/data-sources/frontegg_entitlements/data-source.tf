data "frontegg_entitlements" "for_tenant" {
  tenant_ids   = ["11111111-1111-1111-1111-111111111111"]
  assign_level = "TENANT"
}

output "tenant_entitlement_ids" {
  value = [for e in data.frontegg_entitlements.for_tenant.entitlements : e.id]
}
