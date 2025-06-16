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

  saml {
    acs_url      = "https://mycompany.com/saml"
    sp_entity_id = "my-company"
    redirect_url = "http://localhost:3000"
  }

  oidc {
    redirect_url = "http://localhost:3000"
  }

  sso_multi_tenant_policy {
    unspecified_tenant_strategy = "BLOCK"
    use_active_tenant           = false
  }
}
