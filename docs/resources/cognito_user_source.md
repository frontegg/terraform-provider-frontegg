---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "frontegg_cognito_user_source Resource - terraform-provider-frontegg"
subcategory: ""
description: |-
  Configures a Frontegg Cognito user source.
---

# frontegg_cognito_user_source (Resource)

Configures a Frontegg Cognito user source.

## Example Usage

```terraform
resource "frontegg_cognito_user_source" "example" {
  name                 = "Example Cognito User Source"
  description          = "An example Cognito user source"
  index                = 1
  sync_on_login        = true
  is_migrated          = false
  region               = "us-west-2"
  client_id            = "cognito-client-id"
  user_pool_id         = "us-west-2_abcdefghi"
  access_key_id        = "AKIAIOSFODNN7EXAMPLE"
  secret_access_key    = "SECRET_ACCESS_KEY"
  client_secret        = "cognito-client-secret"
  tenant_resolver_type = "static"
  tenant_id            = "tenant-1234567890"

  app_ids = [
    "app-1234567890"
  ]
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `access_key_id` (String) The access key of the AWS account.
- `client_id` (String) The Cognito app client ID.
- `index` (Number) The user source index.
- `name` (String) The user source name.
- `region` (String) The AWS region of the Cognito user pool.
- `secret_access_key` (String, Sensitive) The secret of the AWS account.
- `tenant_resolver_type` (String) The tenant resolver type (dynamic, static, or new).
- `user_pool_id` (String) The ID of the Cognito user pool.

### Optional

- `app_ids` (Set of String) The application IDs to assign to this user source.
- `client_secret` (String, Sensitive) The Cognito application client secret, required if the app client is configured with a client secret.
- `description` (String) The user source description.
- `is_migrated` (Boolean) Whether to migrate the users.
- `sync_on_login` (Boolean) Whether to sync user profile attributes on each login.
- `tenant_id` (String) The tenant ID for static tenant resolver type.
- `tenant_id_field_name` (String) The attribute name from which the tenant ID would be taken for dynamic tenant resolver type.

### Read-Only

- `id` (String) The ID of this resource.
