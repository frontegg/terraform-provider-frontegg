terraform {
  required_providers {
    frontegg = {
      source = "frontegg/frontegg"
    }
  }
}

provider "frontegg" {
  # client_id and secret_key come from FRONTEGG_CLIENT_ID / FRONTEGG_SECRET_KEY.
}

# custom_domains is an attribute of the workspace, not a resource of its own, so
# every brand's auth host has to be aggregated into this one set. Splitting it per
# brand would leave several resources fighting over the same attribute.
resource "frontegg_workspace" "this" {
  name                = var.workspace_name
  country             = var.country
  backend_stack       = var.backend_stack
  frontend_stack      = var.frontend_stack
  open_saas_installed = false
  frontegg_domain     = var.frontegg_domain
  custom_domains      = [for brand in var.brands : brand.auth_host]
  allowed_origins     = [for brand in var.brands : brand.app_url]

  # The workspace resource requires these policy blocks. They are fleet-wide rather
  # than per-brand; the values here are only placeholders to make the example apply.
  mfa_policy {
    allow_remember_device = true
    device_expiration     = 604800
    enforce               = "unless-saml"
  }

  mfa_authentication_app {
    service_name = var.workspace_name
  }

  lockout_policy {
    max_attempts = 10
  }

  password_policy {
    allow_passphrases = true
    min_length        = 10
    max_length        = 128
    min_tests         = 2
    min_phrase_length = 6
    history           = 2
  }
}

module "brand" {
  for_each = var.brands
  source   = "./modules/brand"

  key                 = each.key
  name                = each.value.name
  auth_host           = each.value.auth_host
  app_url             = each.value.app_url
  mobile_bundle_ids   = each.value.mobile_bundle_ids
  extra_redirect_uris = each.value.extra_redirect_uris
  metadata            = each.value.metadata

  depends_on = [frontegg_workspace.this]
}
