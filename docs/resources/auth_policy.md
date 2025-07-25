---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "frontegg_auth_policy Resource - terraform-provider-frontegg"
subcategory: ""
description: |-
  Configures the general authentication policy for the workspace.
  This is a singleton resource. You must only create one frontegg_auth_policy resource
  per Frontegg provider.
  Note: This resource cannot be deleted. When destroyed, Terraform will remove it from the state file, but the authentication policy will remain in its last-applied state.
---

# frontegg_auth_policy (Resource)

Configures the general authentication policy for the workspace.

This is a singleton resource. You must only create one frontegg_auth_policy resource
per Frontegg provider.

**Note:** This resource cannot be deleted. When destroyed, Terraform will remove it from the state file, but the authentication policy will remain in its last-applied state.

## Example Usage

```terraform
resource "frontegg_auth_policy" "example" {
  allow_unverified_users           = true
  allow_signups                    = true
  allow_tenant_invitations         = true
  enable_api_tokens                = true
  machine_to_machine_auth_strategy = "ClientCredentials"
  enable_roles                     = true
  jwt_algorithm                    = "RS256"
  jwt_access_token_expiration      = 86400   # 1 day
  jwt_refresh_token_expiration     = 2592000 # 30 days
  same_site_cookie_policy          = "strict"
  auth_strategy                    = "EmailAndPassword"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `allow_signups` (Boolean) Whether users are allowed to sign up.
- `allow_tenant_invitations` (Boolean) Allow tenants to invite new users via an invitation link.
- `allow_unverified_users` (Boolean) Whether unverified users are allowed to log in.
- `auth_strategy` (String) The authentication strategy to use for people logging in.

Must be one of "EmailAndPassword", "Code", "MagicLink", "NoLocalAuthentication", "SmsCode"
- `enable_api_tokens` (Boolean) Whether users can create API tokens.
- `enable_roles` (Boolean) Whether granular roles and permissions are enabled.
- `jwt_access_token_expiration` (Number) The expiration time for the JWT access tokens issued by Frontegg.
- `jwt_refresh_token_expiration` (Number) The expiration time for the JWT refresh tokens issued by Frontegg.
- `same_site_cookie_policy` (String) The SameSite policy to use for Frontegg cookies.

Must be one of "none", "lax", or "strict".

### Optional

- `jwt_algorithm` (String) The algorithm Frontegg uses to sign JWT tokens.
- `machine_to_machine_auth_strategy` (String) Type of tokens users will be able to generate.
				Must be one of "ClientCredentials" or "AccessToken".

### Read-Only

- `id` (String) The ID of this resource.
- `jwt_public_key` (String) The public key that Frontegg uses to sign JWT tokens.
