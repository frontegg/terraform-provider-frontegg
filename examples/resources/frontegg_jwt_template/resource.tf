resource "frontegg_jwt_template" "example" {
  key         = "example-template"
  name        = "Example JWT Template"
  description = "An example JWT template with custom claims"
  expiration  = 3600
  algorithm   = "RS256"

  # Frontegg requires the standard OIDC claims (iss, sub, aud, exp, iat) in every
  # template, where aud must be {{clientId}} or {{applicationId}}. Additional
  # custom claims may be added alongside them.
  #
  # Claims are NOT auto-populated: whatever this map contains is what the token
  # carries. In particular, set tenantId yourself if your application or the
  # Frontegg frontend SDK expects it — tokens issued without it break the SDK.
  #
  # A small set of claims is reserved for internal use and rejected by the API
  # (for example type, userId, superUser, act, amr, acr, auth_time, nonce).
  claims = {
    iss      = "{{iss}}"
    sub      = "{{sub}}"
    aud      = "{{clientId}}"
    exp      = "{{exp}}"
    iat      = "{{iat}}"
    tenantId = "{{user.tenantId}}"
    email    = "{{user.email}}"
  }
}

# Use claims_json instead of claims when a claim value is not a string, such as
# a nested object. Exactly one of claims or claims_json may be set.
resource "frontegg_jwt_template" "nested_claims" {
  key        = "nested-claims-template"
  name       = "Nested Claims JWT Template"
  expiration = 3600
  algorithm  = "RS256"

  claims_json = jsonencode({
    iss      = "{{iss}}"
    sub      = "{{sub}}"
    aud      = "{{clientId}}"
    exp      = "{{exp}}"
    iat      = "{{iat}}"
    tenantId = "{{user.tenantId}}"
    organization = {
      id    = "{{user.tenantId}}"
      email = "{{user.email}}"
    }
  })
}
