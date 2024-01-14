# Terraform Provider for Frontegg

This repository contains a Terraform provider for the [Frontegg] user management
platform.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0.3
- [Go](https://golang.org/doc/install) >= 1.20
  **pay attention to install platform compatible version**

## Using the provider

See the Terraform Registry: <https://registry.terraform.io/providers/frontegg/frontegg/latest>.

## Importing existing resources

### Workspaces

To import an existing workspace, first add a shim resource definition to your
Terraform project:

```tf
# main.tf
resource "frontegg_workspace" "example" {}
```

Then run `terraform import`, specifying the address of the resource you declared
above (`frontegg_workspace.example`) and your workspace ID (i.e., your API
client ID):

```shell
$ terraform import frontegg_workspace.example 65e2d503-c187-4d55-8ba5-816bd4a15f96
frontegg_workspace.example: Importing from ID "65e2d503-c187-4d55-8ba5-816bd4a15f96"...
frontegg_workspace.example: Import prepared!
  Prepared frontegg_workspace for import
frontegg_workspace.example: Refreshing state... [id=65e2d503-c187-4d55-8ba5-816bd4a15f96]

Import successful!

The resources that were imported are shown above. These resources are now in
your Terraform state and will henceforth be managed by Terraform.
```

Next, run `terraform state show` to show the configuration values Terraform has
imported:

```shell
$ terraform state show frontegg_workspace.example
# frontegg_workspace.example:
resource "frontegg_workspace" "example" {
    allowed_origins     = [
        "https://yourcompany.com",
    ]
    backend_stack       = "Python"
    country             = "US"
    frontegg_domain     = "yourcompany.frontegg.com"
    # ...
}
```

Finally, you can copy that output back into your `main.tf` file (or equivalent).
Beware that you may need to manually remove some output properties from the
resource, like `jwt_public_key`.

You should verify that `terraform plan` reports no diffs.

### Roles, permissions, and permission categories

The procedure is the same as above, except that it is tricky to discover the ID
for the role, permission, or permission category. IDs for these objects are
UUIDs.

You can either query the [Frontegg API](https://docs.frontegg.com/reference)
yourself to find these IDs, or you can use your browser's developer tools to
sniff the IDs out of the network requests as you browse the [Frontegg
Portal](https://portal.frontegg.com).

### Contact us

Please note that this provider may not offer full support for all Frontegg capabilities. If you require assistance or support for a specific functionality, please contact us at support@frontegg.com.
