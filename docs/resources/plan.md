---
page_title: "frontegg_plan Resource - terraform-provider-frontegg"
subcategory: ""
description: |-
  Manages a Frontegg plan.
---

# frontegg_plan (Resource)

This resource allows you to create, update, and delete plans in your Frontegg environment. Plans are used to manage feature entitlements for your users and tenants.

## Example Usage

```terraform
resource "frontegg_plan" "basic_plan" {
  name                     = "Basic Plan"
  description              = "Basic plan with limited features"
  default_treatment        = "true"
  assign_on_signup         = true
  default_time_limitation  = 30 # 30 days
}
```

## Argument Reference

The following arguments are supported:

* `name` - (Required) The name of the plan.
* `description` - (Optional) A description of the plan.
* `default_treatment` - (Optional) The default treatment for the plan. Must be either "true" or "false".
* `assign_on_signup` - (Optional) Whether the plan is assigned automatically upon signup. Defaults to `false`.
* `default_time_limitation` - (Optional) Default time limitation in days for auto-assigned plans. Set to 0 for no time limitation.
* `rules` - (Optional) Set of conditions targeting the plan.
* `feature_keys` - (Optional) Array of feature keys to be applied on the plan.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the plan.
* `vendor_id` - The vendor ID for the plan.
* `created_at` - When the plan was created.
* `updated_at` - When the plan was last updated.

## Import

Plans can be imported using the plan ID:

```bash
terraform import frontegg_plan.example <plan-id>
``` 