resource "frontegg_social_login" "google" {
  provider_name = "google"
  client_id     = "your-google-client-id"
  redirect_url  = "https://yourcompany.com/auth/google/callback"
  secret        = "your-google-secret"
  customised    = true
}

resource "frontegg_social_login" "github" {
  provider_name = "github"
  client_id     = "your-github-client-id"
  redirect_url  = "https://yourcompany.com/auth/github/callback"
  secret        = "your-github-secret"
  customised    = true
}

resource "frontegg_social_login" "facebook" {
  provider_name = "facebook"
  client_id     = "your-facebook-app-id"
  redirect_url  = "https://yourcompany.com/auth/facebook/callback"
  secret        = "your-facebook-app-secret"
  customised    = true
}

resource "frontegg_social_login" "microsoft" {
  provider_name     = "microsoft"
  client_id         = "your-microsoft-application-id"
  redirect_url      = "https://yourcompany.com/auth/microsoft/callback"
  secret            = "your-microsoft-client-secret"
  customised        = true
  additional_scopes = ["User.Read", "profile"]
}
