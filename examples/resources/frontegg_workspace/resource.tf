resource "frontegg_workspace" "example" {
  name                = "Your Company"
  country             = "US"
  backend_stack       = "Python"
  frontend_stack      = "React"
  open_saas_installed = false

  # If you've configured CNAME record,
  # you can use that custom domain like so:
  # custom_domains = ["frontegg.yourcompany.com"]

  frontegg_domain = "blah.frontegg.com"
  allowed_origins = ["https://yourcompany.com"]

  auth_policy {
    allow_unverified_users           = true
    allow_signups                    = true
    enable_api_tokens                = true
    enable_roles                     = true
    jwt_algorithm                    = "RS256"
    machine_to_machine_auth_strategy = "ClientCredentials"
    jwt_access_token_expiration      = 86400   # 1 day
    jwt_refresh_token_expiration     = 2592000 # 30 days
    same_site_cookie_policy          = "strict"
    auth_strategy                    = "EmailAndPassword"
    allow_tenant_invitations         = true
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
    customised   = false
  }

  github_social_login {
    client_id    = "fake-client-id"
    redirect_url = "fake-redirect-url"
    secret       = "fake-secret"
    customised   = false
  }

  google_social_login {
    client_id    = "fake-client-id"
    redirect_url = "fake-redirect-url"
    secret       = "fake-secret"
    customised   = false
  }

  microsoft_social_login {
    client_id    = "fake-client-id"
    redirect_url = "fake-redirect-url"
    secret       = "fake-secret"
    customised   = false
  }

  saml {
    acs_url      = "https://mycompany.com/saml"
    sp_entity_id = "my-company"
    redirect_url = "http://localhost:3000"
  }

  oidc {
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
    enable_groups              = false
    enable_provisioning        = false

    palette {
      error {
        contrast_text = "#eeeef0"
        dark          = "#ae402c"
        light         = "#FFEEEA"
        main          = "#E1583E"
      }
      info {
        contrast_text = "#eeeef0"
        dark          = "#3c6492"
        light         = "#E2EEF9"
        main          = "#5587C0"
      }
      primary {
        active        = "#278854"
        contrast_text = "#eeeef0"
        dark          = "#36A76A"
        hover         = "#32A265"
        light         = "#A2E1BF"
        main          = "#43BB7A"
      }
      secondary {
        active        = "#E6ECF4"
        contrast_text = "#eeeef0"
        dark          = "#E6ECF4"
        hover         = "#F0F3F8"
        light         = "#FBFBFC"
        main          = "#FBFBFC"
      }
      success {
        contrast_text = "#eeeef0"
        dark          = "#1d7c30"
        light         = "#E1F5E2"
        main          = "#2CA744"
      }
      warning {
        contrast_text = "#eeeef0"
        dark          = "#EAE1C2"
        light         = "#F9F4E2"
        main          = "#A79D7B"
      }
    }
  }
}
