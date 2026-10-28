---
layout: ""
page_title: "Keeping Secrets Out of Terraform State"
description: |-
  How to supply Frontegg secrets through write-only arguments so they are never stored in the Terraform plan or state.
---

# Keeping Secrets Out of Terraform State

Every secret attribute in this provider is marked sensitive, so Terraform redacts it in plan
and apply output. Sensitive values are still written to the state file in plain text. To keep
a secret out of state completely, supply it through the matching write-only argument.

## Requirements

Write-only arguments need **Terraform 1.11 or later**. Ephemeral input variables and ephemeral
resources, the usual source of a write-only value, need Terraform 1.10 or later.

The rest of the provider still supports Terraform 1.0.3 and later. On versions before 1.11,
keep using the regular secret arguments. Terraform rejects a configuration that sets a
write-only argument when it runs on a version that does not support them.

## Write-only arguments

Each secret below has a `<name>_wo` argument that accepts the value without storing it,
and a `<name>_wo_version` argument that tells Terraform when to send it.

| Resource                          | Secret              | Write-only argument    | Version argument               |
|-----------------------------------|---------------------|------------------------|--------------------------------|
| `frontegg_webhook`                | `secret`            | `secret_wo`            | `secret_wo_version`            |
| `frontegg_prehook`                | `secret`            | `secret_wo`            | `secret_wo_version`            |
| `frontegg_email_provider`         | `secret`            | `secret_wo`            | `secret_wo_version`            |
| `frontegg_secret`                 | `value`             | `value_wo`             | `value_wo_version`             |
| `frontegg_social_login`           | `secret`            | `secret_wo`            | `secret_wo_version`            |
| `frontegg_tenant_oidc_config`     | `oidc_secret`       | `oidc_secret_wo`       | `oidc_secret_wo_version`       |
| `frontegg_tenant_saml_config`     | `idp_client_secret` | `idp_client_secret_wo` | `idp_client_secret_wo_version` |
| `frontegg_auth0_user_source`      | `secret`            | `secret_wo`            | `secret_wo_version`            |
| `frontegg_cognito_user_source`    | `secret_access_key` | `secret_access_key_wo` | `secret_access_key_wo_version` |
| `frontegg_cognito_user_source`    | `client_secret`     | `client_secret_wo`     | `client_secret_wo_version`     |
| `frontegg_firebase_user_source`   | `private_key`       | `private_key_wo`       | `private_key_wo_version`       |
| `frontegg_federation_user_source` | `secret`            | `secret_wo`            | `secret_wo_version`            |

A secret and its write-only argument are mutually exclusive. Where the secret was required,
exactly one of the two must be set. The write-only argument and its version must be set
together.

## Example

```terraform
variable "webhook_secret" {
  type      = string
  ephemeral = true
}

resource "frontegg_webhook" "example" {
  enabled     = true
  description = "Signed with a secret that never reaches state"
  url         = "https://example.com/webhook"
  events      = ["frontegg.user.authenticated"]

  secret_wo         = var.webhook_secret
  secret_wo_version = 1
}
```

## Rotating a write-only secret

Terraform does not store a write-only value, so it cannot tell when the value changes.
Changing `secret_wo` alone produces no plan. To send a new value, change the secret and
increase `secret_wo_version` in the same apply:

```terraform
  secret_wo         = var.webhook_secret
  secret_wo_version = 2
```

## Moving an existing secret to a write-only argument

Replace the secret argument with its write-only pair and apply. The provider sends the
write-only value and removes the old value from state.

## Importing resources with secrets

An import never reads a secret into state, because `terraform import` has no configuration
to tell whether the secret is supplied write-only. Either way, the first plan after import
shows a one-time update. With a write-only pair, it records `*_wo_version` and sends the
write-only value. With a plain secret argument, it sends the configured secret, and after
that apply, refresh tracks the secret as before.
