resource "frontegg_firebase_user_source" "example" {
  name                 = "Example Firebase User Source"
  description          = "An example Firebase user source"
  index                = 1
  sync_on_login        = true
  is_migrated          = false
  tenant_resolver_type = "static"
  tenant_id            = "tenant-1234567890"

  api_key                     = "API_KEY"
  service_account_type        = "service_account"
  project_id                  = "example-project-123456"
  private_key_id              = "1234567890abcdef1234567890abcdef12345678"
  private_key                 = "-----BEGIN PRIVATE KEY-----\nEXAMPLE_PRIVATE_KEY\n-----END PRIVATE KEY-----\n"
  client_email                = "firebase-adminsdk-abc123@example-project-123456.iam.gserviceaccount.com"
  client_id                   = "123456789012345678901"
  auth_uri                    = "https://accounts.google.com/o/oauth2/auth"
  token_uri                   = "https://oauth2.googleapis.com/token"
  auth_provider_x509_cert_url = "https://www.googleapis.com/oauth2/v1/certs"
  client_x509_cert_url        = "https://www.googleapis.com/robot/v1/metadata/x509/firebase-adminsdk-abc123%40example-project-123456.iam.gserviceaccount.com"
  universe_domain             = "googleapis.com"

  app_ids = [
    "app-1234567890"
  ]
}
