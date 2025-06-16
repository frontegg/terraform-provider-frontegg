resource "frontegg_admin_portal" "example" {
  enable_account_settings    = true
  enable_api_tokens          = true
  enable_audit_logs          = true
  enable_personal_api_tokens = true
  enable_privacy             = true
  enable_profile             = true
  enable_roles               = true
  enable_security            = true
  enable_sso                 = true
  enable_subscriptions       = true
  enable_usage               = true
  enable_users               = true
  enable_webhooks            = true
  enable_groups              = true
  enable_provisioning        = true

  # Optional: Configure custom palette for admin portal
  palette {
    error {
      contrast_text = "#ffffff"
      dark          = "#ae402c"
      light         = "#FFEEEA"
      main          = "#E1583E"
    }
    info {
      contrast_text = "#ffffff"
      dark          = "#3c6492"
      light         = "#E2EEF9"
      main          = "#5587C0"
    }
    primary {
      active        = "#278854"
      contrast_text = "#ffffff"
      dark          = "#36A76A"
      hover         = "#32A265"
      light         = "#A2E1BF"
      main          = "#43BB7A"
    }
    secondary {
      active        = "#E6ECF4"
      contrast_text = "#000000"
      dark          = "#E6ECF4"
      hover         = "#F0F3F8"
      light         = "#FBFBFC"
      main          = "#FBFBFC"
    }
    success {
      contrast_text = "#ffffff"
      dark          = "#1d7c30"
      light         = "#E1F5E2"
      main          = "#2CA744"
    }
    warning {
      contrast_text = "#ffffff"
      dark          = "#d4a017"
      light         = "#F9F4E2"
      main          = "#f4c430"
    }
  }
}
