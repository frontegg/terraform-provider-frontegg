terraform {
  required_providers {
    frontegg = {
      source = "frontegg/frontegg"
    }
  }
}

provider "frontegg" {
  api_base_url = "https://api.stg.frontegg.com"
  client_id  = "7c1908a7-e49a-49bd-b007-8b25c5dd1780"
  secret_key = "9f8dc1a4-1721-442e-bbe0-ca762ed39458"
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
      error {
        contrast_text= ""
        dark= "#ae402c"
        light= "#FFEEEA"
        main= "#E1583E"
      }
      info {
        contrast_text= ""
        dark= "#3c6492"
        light= "#E2EEF9"
        main= "#5587C0"
      }
      primary {
        active = "#278854"
        contrast_text = ""
        dark = "#36A76A"
        hover = "#32A265"
        light = "#A2E1BF"
        main = "#43BB7A"
      }
      secondary {
        active = "#E6ECF4"
        contrast_text = ""
        dark = "#E6ECF4"
        hover = "#F0F3F8"
        light = "#FBFBFC"
        main = "#FBFBFC"
      }
      success {
        contrast_text = ""
        dark = "#1d7c30"
        light = "#E1F5E2"
        main = "#2CA744"
      }
      warning {
        contrast_text = ""
        dark = "#EAE1C2"
        light = "#F9F4E2"
        main = "#A79D7B"
      }
    }
  }
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
  default     = true
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

