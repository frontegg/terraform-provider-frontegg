resource "frontegg_jwt_template" "example" {
  key         = "example-template"
  name        = "Example JWT Template"
  description = "An example JWT template with custom claims"
  expiration  = 3600
  algorithm   = "RS256"

  # Frontegg requires the standard OIDC claims (iss, sub, aud, exp, iat) and the
  # Frontegg claims (type, tenantId) in every template. Additional custom claims
  # may be added alongside them.
  claims = {
    iss      = "{{iss}}"
    sub      = "{{sub}}"
    aud      = "{{aud}}"
    exp      = "{{exp}}"
    iat      = "{{iat}}"
    type     = "{{type}}"
    tenantId = "{{user.tenantId}}"
    email    = "{{user.email}}"
  }
}
