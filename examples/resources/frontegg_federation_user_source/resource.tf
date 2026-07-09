# OIDC variant: configure via the provider's well-known discovery URL.
resource "frontegg_federation_user_source" "oidc_example" {
  name                 = "Example OIDC Federation User Source"
  description          = "An example OIDC federation user source"
  sync_on_login        = true
  tenant_resolver_type = "static"
  tenant_id            = "tenant-1234567890"

  wellknown_url = "https://idp.example.com/.well-known/openid-configuration"
  client_id     = "client-1234567890"
  secret        = "client-secret"

  app_ids = [
    "app-1234567890"
  ]
}

# OAuth2 variant: configure the endpoints explicitly and use PKCE (no secret).
resource "frontegg_federation_user_source" "oauth2_example" {
  name                 = "Example OAuth2 Federation User Source"
  description          = "An example OAuth2 federation user source"
  sync_on_login        = true
  tenant_resolver_type = "new"

  client_id = "client-1234567890"
  use_pkce  = true

  oauth2_config {
    authorization_url      = "https://idp.example.com/authorize"
    token_url              = "https://idp.example.com/token"
    user_info_url          = "https://idp.example.com/userinfo"
    scopes                 = ["openid", "email", "profile"]
    grant_types            = ["authorization_code"]
    code_challenge_methods = ["S256"]
  }
}
