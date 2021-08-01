terraform {
  required_providers {
    frontegg = {
      source = "benesch/frontegg"
    }
  }
}

resource "frontegg_workspace" "example" {
  name                = "Your Company"
  country             = "US"
  backend_stack       = "Python"
  frontend_stack      = "React"
  open_saas_installed = false

  frontegg_domain = "example.frontegg.com"
  allowed_origins = ["https://yourcompany.com"]

  auth_policy {
    allow_unverified_users       = false
    allow_signups                = true
    enable_api_tokens            = true
    enable_roles                 = true
    jwt_algorithm                = "RS256"
    jwt_access_token_expiration  = 86400   # 1 day
    jwt_refresh_token_expiration = 2592000 # 30 days
    same_site_cookie_policy      = "strict"
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

  captcha_policy {
    site_key   = "fake-site-key"
    secret_key = "fake-secret-key"
    min_score  = 0.5
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

resource "frontegg_webhook" "example" {
  enabled     = true
  name        = "Example webhook"
  description = "An example of a webhook"
  url         = "https://example.com/webhook"
  secret      = "example-sekret"
  events = [
    "frontegg.user.authenticated"
  ]
}

resource "frontegg_permission_category" "example" {
  name        = "Example"
  description = "An example of a permission category"
}

resource "frontegg_permission" "example" {
  name        = "Example"
  key         = "example"
  description = "An example of a permission"
  category_id = resource.frontegg_permission_category.example.id
}

data "frontegg_permission" "read_users" {
  key = "fe.secure.read.users"
}

resource "frontegg_role" "example" {
  name        = "Example"
  key         = "example"
  description = "An example of a role"
  level       = 0
  permission_ids = [
    resource.frontegg_permission.example.id,
    data.frontegg_permission.read_users.id,
  ]
}

output "public_key" {
  value = resource.frontegg_workspace.example.auth_policy.0.jwt_public_key
}