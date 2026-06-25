resource "frontegg_jwt_template" "example" {
  key         = "example-template"
  name        = "Example JWT Template"
  description = "An example JWT template with custom claims"
  expiration  = 3600
  algorithm   = "RS256"

  # Frontegg requires the standard OIDC claims (iss, sub, aud, exp, iat) in every
  # template, where aud must be {{clientId}} or {{applicationId}}. Additional
  # custom claims may be added alongside them. Claims reserved for internal use
  # (e.g. type, tenantId) are populated by Frontegg and must not be set here.
  claims = {
    iss   = "{{iss}}"
    sub   = "{{sub}}"
    aud   = "{{clientId}}"
    exp   = "{{exp}}"
    iat   = "{{iat}}"
    email = "{{user.email}}"
  }
}
