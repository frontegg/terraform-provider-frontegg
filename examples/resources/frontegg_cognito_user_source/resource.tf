resource "frontegg_cognito_user_source" "example" {
  name                 = "Example Cognito User Source"
  description          = "An example Cognito user source"
  index                = 1
  sync_on_login        = true
  is_migrated          = false
  region               = "us-west-2"
  client_id            = "cognito-client-id"
  user_pool_id         = "us-west-2_abcdefghi"
  access_key_id        = "AKIAIOSFODNN7EXAMPLE"
  secret_access_key    = "SECRET_ACCESS_KEY"
  client_secret        = "cognito-client-secret"
  tenant_resolver_type = "static"
  tenant_id            = "tenant-1234567890"

  app_ids = [
    "app-1234567890"
  ]
}
