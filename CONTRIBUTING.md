# Contributing

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

### Iteration cycle

This provider does not yet have an acceptance test suite. Instead, developers
test changes manually by editing the basic example and running `terraform
apply`. Following are the commands to run to efficiently manually test your
changes.

```
export FRONTEGG_CLIENT_ID=<redacted>
export FRONTEGG_SECRET_KEY=<redacted>
cd examples/basic
# Recompile the provider, reinitialize Terraform, then apply the current state.
# Run repeatedly as you make changes to the provider or to the basic example.
(cd ../.. && make install) && rm .terraform.lock.hcl && terraform init && terraform apply -auto-approve
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
