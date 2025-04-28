resource "frontegg_auth0_user_source" "example" {
  name                = "Example Auth0 User Source"
  description         = "An example Auth0 user source"
  index               = 1
  sync_on_login       = true
  is_migrated         = false
  domain              = "example.auth0.com"
  client_id           = "auth0-client-id"
  secret              = "auth0-client-secret"
  tenant_resolver_type = "static"
  tenant_id           = "tenant-1234567890"
  
  app_ids = [
    "app-1234567890"
  ]
}
