resource "frontegg_jwt_template" "example" {
  key         = "example-template"
  name        = "Example JWT Template"
  description = "An example JWT template with custom claims"
  expiration  = 3600
  algorithm   = "RS256"

  claims = {
    sub      = "{{user.id}}"
    email    = "{{user.email}}"
    tenantId = "{{tenant.id}}"
  }
}
