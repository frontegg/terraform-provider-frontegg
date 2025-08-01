---
layout: ""
page_title: "Provider: Frontegg"
description: |-
  The Frontegg provider provides resources to interact with the Frontegg user
  management platform.
---

# Frontegg Provider

The Frontegg provider provides resources to interact with the [Frontegg] user
management platform.

The provider works with only one workspace at a time. To provision multiple
workspaces, you will need to configure multiple copies of the provider.

Note that the client ID and secret key are *not* the client ID and secret key
that appear in "Workspace Settings". You need to generate a workspace API key
and secret specifically for the Terraform provider's use in the administration
portal:

![API key generation example](https://user-images.githubusercontent.com/882976/132739276-bc72aa75-8c30-452c-b929-85a8d7ffa4d0.png)

In order to interact with specific environment management capabilities you can
provide environment ID, that is displayed on environment settings at the Frontegg
portal. To configure multiple environments you will need to configure multiple
copies of provider with one environment ID per each. If no environment ID was
provided the configuration will be cross-environments.

## Example Usage

```terraform
provider "frontegg" {
  client_id      = "[your-personal-token-client-id]"
  secret_key     = "[your-personal-token-api-key]"
  environment_id = "[your-environment-id]"

  api_base_url    = "https://api.frontegg.com"
  portal_base_url = "https://frontegg-portal.frontegg.com"
}
```

## Migration Guide

If you're upgrading from v1.0.x to v2.0.0, please see the [Migration Guide](guides/migration-v2) for detailed instructions on handling breaking changes and resource restructuring.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `client_id` (String) The client ID for a Frontegg portal API key.
- `secret_key` (String, Sensitive) The corresponding secret key for the API key.

### Optional

- `api_base_url` (String) The Frontegg api url. Override to change region. Defaults to EU url.
- `environment_id` (String, Sensitive) The client ID from environment settings.
- `portal_base_url` (String) The Frontegg portal url. Override to change region. Defaults to EU url.

[Frontegg]: https://frontegg.com
