# White-label fleet onboarding

Onboarding a brand in a white-label fleet means repeating the same set of steps —
custom domain, tenant, application, allowed origin, redirect URIs — once per brand.
Done by hand that is several portal visits each, and the cost scales with the number
of brands rather than staying flat.

This example collapses those steps into a single map entry per brand.

```hcl
brands = {
  northwind = {
    name      = "Northwind Clinic"
    auth_host = "auth.northwind.example.com"
    app_url   = "https://app.northwind.example.com"

    mobile_bundle_ids = {
      ios     = ["com.example.northwind"]
      android = ["com.example.northwind"]
    }
  }
}
```

Adding a brand is adding an entry. Nothing else in the configuration changes.

## Usage

```bash
cp terraform.tfvars.example terraform.tfvars
export FRONTEGG_CLIENT_ID=... FRONTEGG_SECRET_KEY=...
terraform init
terraform plan
```

## Why custom domains live in the root, not the module

`custom_domains` is an *attribute* of the single `frontegg_workspace` resource, not a
resource per domain. Terraform cannot have several resources managing one attribute —
they would each plan to overwrite the others on every apply.

So the workspace aggregates every brand's host:

```hcl
custom_domains = [for brand in var.brands : brand.auth_host]
```

and the per-brand module takes `auth_host` as an input it *uses* but does not own.
This is the one place the fleet cannot be expressed as "everything for a brand lives in
the brand module", and it is worth understanding before extending this example.

The DNS side is still yours: each `auth_host` needs a CNAME pointing at the environment,
and Frontegg will report the domain as `Pending` until that record resolves.

## Mobile redirect URIs

The Frontegg mobile SDKs do not read their OAuth callback from configuration. They
derive it from the auth host:

```
https://{auth_host}/oauth/account/redirect/{ios|android}/{bundleId}
```

The app cannot be told to use anything else, so this exact URI has to be registered or
authorization fails with `Redirect uri wasn't found` — and it fails *after* the user has
already authenticated, which makes it look like a broken login rather than a missing
setting.

The module derives those URIs from `auth_host` and `mobile_bundle_ids` so they cannot
drift from what the SDK actually sends. List every bundle identifier and package name
that ships for the brand.

## What this example does not cover

- **Association files.** The `apple-app-site-association` and `assetlinks.json` files
  that bind a mobile app to its auth host are served by Frontegg, not managed here.
  Confirm they are served on your custom domains before relying on Universal Links or
  App Links for a brand.
- **Passkeys.** WebAuthn `rp.id` is derived by the identity service, not set through
  this provider. If you intend to scope passkeys per brand, settle that *before* a
  brand's first passkey enrolment — changing `rp.id` afterwards orphans credentials
  already registered against the old value.
- **Per-brand branding and email templates.** `frontegg_email_template` and the admin
  portal resources exist and can be folded into the brand module if you want them
  versioned alongside the rest.
