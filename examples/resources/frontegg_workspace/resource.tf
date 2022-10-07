resource "frontegg_workspace" "example" {
  name                = "Your Company"
  country             = "US"
  backend_stack       = "Python"
  frontend_stack      = "React"
  open_saas_installed = false

  # If you've configured a CNAME record to point a domain to "ssl.frontegg.com",
  # you can use that custom domain like so:
  # custom_domain = "frontegg.yourcompany.com"

  frontegg_domain = "blah.frontegg.com"
  allowed_origins = ["https://yourcompany.com"]

  auth_policy {
    allow_unverified_users       = true
    allow_signups                = true
    enable_api_tokens            = true
    enable_roles                 = true
    jwt_algorithm                = "RS256"
    jwt_access_token_expiration  = 86400   # 1 day
    jwt_refresh_token_expiration = 2592000 # 30 days
    same_site_cookie_policy      = "strict"
    auth_strategy                = "Code"
  }

  mfa_policy {
    allow_remember_device = true
    device_expiration     = 604800 # 7 days
    enforce               = "unless-saml"
  }

  mfa_authentication_app {
    service_name = "Your Company"
  }

  lockout_policy {
    max_attempts = 10
  }

  password_policy {
    allow_passphrases = false
    min_length        = 10
    max_length        = 128
    min_tests         = 2
    min_phrase_length = 6
    history           = 2
  }

  captcha {
    site_key   = "fake-site-key"
    secret_key = "fake-secret-key"
    min_score  = 0.5
  }

  hosted_login {
    allowed_redirect_urls = [
      "http://example.com/a",
      "http://example.com/b",
    ]
  }

  facebook_social_login {
    client_id    = "fake-client-id"
    redirect_url = "fake-redirect-url"
    secret       = "fake-secret"
  }

  github_social_login {
    client_id    = "fake-client-id"
    redirect_url = "fake-redirect-url"
    secret       = "fake-secret"
  }

  google_social_login {
    client_id    = "fake-client-id"
    redirect_url = "fake-redirect-url"
    secret       = "fake-secret"
  }

  microsoft_social_login {
    client_id    = "fake-client-id"
    redirect_url = "fake-redirect-url"
    secret       = "fake-secret"
  }

  saml {
    acs_url      = "https://mycompany.com/saml"
    sp_entity_id = "my-company"
    redirect_url = "http://localhost:3000"
  }

  reset_password_email {
    from_address  = "me@company.com"
    from_name     = "Your Company"
    subject       = "Reset Your Company Password"
    html_template = "<strong>Reset your password! {{redirectURL}}</strong>"
    redirect_url  = "https://yourcompany.com/reset"
  }

  admin_portal {
    enable_account_settings    = false
    enable_api_tokens          = false
    enable_audit_logs          = false
    enable_personal_api_tokens = false
    enable_privacy             = false
    enable_profile             = false
    enable_roles               = false
    enable_security            = false
    enable_sso                 = false
    enable_subscriptions       = false
    enable_usage               = false
    enable_users               = false
    enable_webhooks            = false

    palette {
      success        = "#2ca744"
      info           = "#5587c0"
      warning        = "#ffc107"
      error          = "#e1583e"
      primary        = "#43bb7a"
      primary_text   = "#ffffff"
      secondary      = "#fbfbfc"
      secondary_text = "#3c4a5a"
    }
  }
}
