---
page_title: "frontegg_feature Resource - terraform-provider-frontegg"
subcategory: ""
description: |-
  Manages a Frontegg feature.
---

# frontegg_feature (Resource)

This resource allows you to create, update, and manage features in Frontegg's Entitlements feature system. Features can be assigned to plans and used in feature flags to control access to functionality in your application.

## Example Usage

```terraform
resource "frontegg_feature" "example" {
  name        = "My Feature"
  key         = "my-feature"
  description = "This is an example feature"
  
  # Optional: Link permissions to this feature
  # Permissions are sorted by permission_key for consistent ordering
  permissions = [
    {
      permission_key = "read.example"
      permission_id  = "permission-id-from-frontegg"
    },
    {
      permission_key = "write.example"
      permission_id  = "another-permission-id"
    }
  ]
  
  # Optional: Add metadata as a map
  metadata = jsonencode({
    category = "core"
    isPublic = true
  })
}
```

## Argument Reference

The following arguments are supported:

* `name` - (Required) The name of the feature.
* `key` - (Required) The unique key identifier for the feature. If a feature with this key already exists, the provider will update that feature instead of creating a new one.
* `description` - (Optional) A description of the feature.
* `permissions` - (Optional) A list of permissions associated with the feature. The permissions are automatically sorted by `permission_key` for consistent ordering. Each element is an object with the following attributes:
  * `permission_key` - (Required) The key of the permission.
  * `permission_id` - (Required) The ID of the permission.
* `metadata` - (Optional) A JSON-encoded string containing additional metadata for the feature. Use `jsonencode()` function to convert a map to the required JSON string format.

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the feature (UUID).
* `created_at` - The timestamp when the feature was created.
* `updated_at` - The timestamp when the feature was last updated.

## Import

Features can be imported using the feature `id`:

```
$ terraform import frontegg_feature.example f6a5012c-cbeb-4c1e-ab80-e5f43efd44e3
```

## Notes

* When creating a feature, if a feature with the same `key` already exists, the provider will update that existing feature instead of creating a new one.
* Permissions are maintained in a sorted order by `permission_key` to ensure consistent state management.
* The metadata should be provided as a JSON-encoded string using the `jsonencode()` function to ensure proper formatting. 