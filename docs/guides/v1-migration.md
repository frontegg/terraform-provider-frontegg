---
page_title: "frontegg_migration-v0 Migration - V0 to V1 migration"
subcategory: ""
description: |-
  Migration guide for the Frontegg provider from V0 to V1.
---

# V1 Migration Guide

This guide will help you migrate from Frontegg's V0 to V1.

> **This migration guide applies exclusively to users who have configured a custom domain with the provider.**

## Provider Configuration

> [!IMPORTANT]
> If you update your provider without proceeding with following guide, you'll get an error when running `terraform apply`.

### Custom Domains

We've made a significant update to the provider configuration, introducing the `custom_domains` field to replace the previous `custom_domain` field. The `custom_domains` field now accepts a list of strings, allowing you to configure multiple custom domains for the Frontegg application.

To implement this update, follow these steps:

1. Navigate to [Frontegg Portal](https://portal.frontegg.com) and initiate the custom domain migration process.
   > For more information, see [Frontegg's custom domain migration guide](https://docs.frontegg.com/docs/custom-domain-migration).
   > [!WARNING]
   > Once you've migrated to the new version, you'll need to update your configuration to use the new `custom_domains` field, or you'll get error when running `terraform apply`.
2. Upon completion of the migration, you'll receive a list of custom domains available for use with the `custom_domains` field.
3. Ensure you're using Provider version V1.
4. Replace any instances of the old `custom_domain` field in your configuration (
   <s>custom_domain = "https://yourcCustomDomain.com"</s>) with the new `custom_domains` field. Populate it with the list of custom domains as follows:

```hcl
custom_domains = [
  "https://yourcCustomDomain.com",
  "https://yourcCustomDomain2.com"
]
```

[Frontegg]: https://frontegg.com
