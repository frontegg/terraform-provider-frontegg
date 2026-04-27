data "frontegg_plan" "basic" {
  name = "Basic Plan"
}

# Compose with frontegg_entitlement to grant a plan by human-readable name.
resource "frontegg_entitlement" "by_name" {
  entitlement {
    plan_id   = data.frontegg_plan.basic.id
    tenant_id = "11111111-1111-1111-1111-111111111111"
  }
}
