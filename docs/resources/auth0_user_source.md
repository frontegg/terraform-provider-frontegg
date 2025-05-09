---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "frontegg_auth0_user_source Resource - terraform-provider-frontegg"
subcategory: ""
description: |-
  Configures a Frontegg Auth0 user source.
---

# frontegg_auth0_user_source (Resource)

Configures a Frontegg Auth0 user source.



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `client_id` (String) The Auth0 application client ID.
- `domain` (String) The Auth0 domain.
- `index` (Number) The user source index.
- `name` (String) The user source name.
- `secret` (String, Sensitive) The Auth0 application secret.
- `tenant_resolver_type` (String) The tenant resolver type (dynamic, static, or new).

### Optional

- `app_ids` (Set of String) The application IDs to assign to this user source.
- `description` (String) The user source description.
- `is_migrated` (Boolean) Whether to migrate the users.
- `sync_on_login` (Boolean) Whether to sync user profile attributes on each login.
- `tenant_id` (String) The tenant ID for static tenant resolver type.
- `tenant_id_field_name` (String) The attribute name from which the tenant ID would be taken for dynamic tenant resolver type.

### Read-Only

- `id` (String) The ID of this resource.
