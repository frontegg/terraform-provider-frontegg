terraform {
  required_providers {
    frontegg = {
      source = "frontegg/frontegg"
    }
  }
}

locals {
  login_url = "https://${var.auth_host}"

  # The callback each Frontegg mobile SDK derives from the auth host. It is not
  # configurable in the app -- the SDK builds this exact string -- so it has to be
  # registered verbatim.
  mobile_redirect_uris = concat(
    [for id in var.mobile_bundle_ids.ios : "https://${var.auth_host}/oauth/account/redirect/ios/${id}"],
    [for id in var.mobile_bundle_ids.android : "https://${var.auth_host}/oauth/account/redirect/android/${id}"],
  )

  redirect_uris = toset(concat(local.mobile_redirect_uris, var.extra_redirect_uris))
}

resource "frontegg_tenant" "this" {
  name              = var.name
  key               = var.key
  application_uri   = var.app_url
  selected_metadata = var.metadata
}

resource "frontegg_application" "this" {
  name        = var.name
  app_url     = var.app_url
  login_url   = local.login_url
  access_type = "FREE_ACCESS"
  is_active   = true
  is_default  = false
  type        = "web"
}

resource "frontegg_application_tenant_assignment" "this" {
  app_id    = frontegg_application.this.id
  tenant_id = frontegg_tenant.this.id
}

resource "frontegg_redirect_uri" "this" {
  for_each = local.redirect_uris

  redirect_uri = each.value
}
