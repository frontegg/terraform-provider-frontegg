resource "frontegg_auth_policy" "example" {
  allow_unverified_users           = true
  allow_signups                    = true
  allow_tenant_invitations         = true
  enable_api_tokens                = true
  machine_to_machine_auth_strategy = "ClientCredentials"
  enable_roles                     = true
  jwt_algorithm                    = "RS256"
  jwt_access_token_expiration      = 86400   # 1 day
  jwt_refresh_token_expiration     = 2592000 # 30 days
  same_site_cookie_policy          = "strict"
  auth_strategy                    = "EmailAndPassword"
}
