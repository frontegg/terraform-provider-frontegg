# Terraform Provider for Frontegg

This repository contains a Terraform provider for the [Frontegg] user management
platform.

## Requirements

* [Terraform](https://www.terraform.io/downloads.html) >= 1.0.3
* [Go](https://golang.org/doc/install) >= 1.16

## Using the provider

See the Terraform Registry: <https://registry.terraform.io/providers/benesch/frontegg/latest>.

## Developing the provider

If you wish to work on the provider, you'll first need
[Go](http://www.golang.org) installed on your machine (see
[Requirements](#requirements) above).

To compile the provider, run `make install`. This will build the provider and
put the provider binary in the correct location within `~/.terraform.d` so that
Terraform can find the plugin.

To generate or update documentation, run `go generate`.

To run the full suite of acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```sh
$ make testacc
```

### Adding dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using
Go modules.

To add a new dependency:

```
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

[Frontegg]: https://frontegg.com