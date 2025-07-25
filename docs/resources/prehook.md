---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "frontegg_prehook Resource - terraform-provider-frontegg"
subcategory: ""
description: |-
  Configures a Frontegg prehook.
---

# frontegg_prehook (Resource)

Configures a Frontegg prehook.



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `description` (String) A human-readable description of the prehook.
- `enabled` (Boolean) Whether the prehook is enabled.
- `events` (Set of String) The name of the event to subscribe to.
- `fail_method` (String) The action to take when the prehook fails.
- `secret` (String) A secret to validate the event with.
- `url` (String) The URL to send events to.

### Optional

- `name` (String) A human-readable name for the prehook.

### Read-Only

- `id` (String) The ID of this resource.
