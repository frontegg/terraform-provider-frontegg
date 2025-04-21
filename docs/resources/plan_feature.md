---
page_title: "frontegg_plan_feature Resource - terraform-provider-frontegg"
subcategory: ""
description: |-
  Manages the relationship between a Frontegg plan and features.
---

# frontegg_plan_feature (Resource)

This resource allows you to link and unlink features to plans in your Frontegg environment. This defines which features are available in each plan.

## Example Usage

```terraform
# Create a plan
resource "frontegg_plan" "basic_plan" {
  name        = "Basic Plan"
  description = "Basic plan with limited features"
}

# Create features
resource "frontegg_feature" "analytics" {
  name        = "Analytics"
  key         = "analytics"
  description = "Access to analytics dashboard"
}

resource "frontegg_feature" "reporting" {
  name        = "Reporting"
  key         = "reporting"
  description = "Access to reporting tools"
}

# Link multiple features to the plan
resource "frontegg_plan_feature" "basic_features" {
  plan_id     = frontegg_plan.basic_plan.id
  feature_ids = [
    frontegg_feature.analytics.id,
    frontegg_feature.reporting.id
  ]
}
```

## Argument Reference

The following arguments are supported:

* `plan_id` - (Required) The ID of the plan.
* `feature_ids` - (Required) A set of feature IDs to link to the plan.

## Import

Plan-feature relationships can be imported using the plan_id:

```bash
terraform import frontegg_plan_feature.example <plan-id>
``` 