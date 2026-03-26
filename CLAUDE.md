# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Terraform provider for the Frontegg user management platform, written in Go. Uses Terraform Plugin SDK v2.

## Requirements

- **Terraform** >= 1.0.3
- **Go** >= 1.20 (install platform-compatible version)
- `FRONTEGG_CLIENT_ID` and `FRONTEGG_SECRET_KEY` env vars for authentication

## Common Commands

```bash
make install     # Build and install provider locally to ~/.terraform.d/plugins
make capply      # make install, then writes ~/.terraform.d/dev_overrides.tfrc, then terraform apply
make testacc     # TF_ACC=1 go test ./... -v -timeout 120m (creates real resources)
make lint        # go vet, gofmt check, go mod tidy
make lint-fix    # go vet, gofmt -w, go mod tidy
go generate      # Regenerate docs via terraform-plugin-docs
```

## Architecture

### Structure

```
main.go                        # plugin.Serve() entry point
provider/
  provider.go                  # Provider config, ResourcesMap, DataSourcesMap
  resource_frontegg_*.go       # Resource CRUD implementations
  data_source_*.go             # Data source implementations
  validators/                  # Custom schema validators
internal/restclient/
  restclient.go                # HTTP client (auth, retry, logging)
  clientholder.go              # Holds API + Portal clients
examples/                      # Example .tf configurations
```

### Dual Client Architecture

Two REST clients are initialized in `provider.go`:
- **API Client**: `api.frontegg.com` — used for most resources
- **Portal Client**: `frontegg-prod.frontegg.com` — used for portal-specific operations

Both authenticate via `/auth/vendor` using the same credentials.

### Provider Configuration

- `client_id` / `FRONTEGG_CLIENT_ID`
- `secret_key` / `FRONTEGG_SECRET_KEY`
- `api_base_url` / `FRONTEGG_API_BASE_URL` (default: EU region)
- `portal_base_url` / `FRONTEGG_PORTAL_BASE_URL` (default: EU region)
- `environment_id` — optional; injected as a header on all requests
- `application_id` — optional; injects `frontegg-application-id` header on all requests

### Non-Obvious Cross-File Patterns

**Shared structs in `resource_frontegg_workspace.go`**: The `fronteggSSODomain` struct and `fronteggSSODomainURL` constant are defined in `resource_frontegg_workspace.go` but reused in `resource_frontegg_sso_domain_policy.go`. Don't define duplicates — import from workspace.

**Tenant-scoped resources**: SSO resources (tenant_sso_domain, tenant_saml_config, tenant_oidc_config, tenant_mfa_policy, etc.) send a `frontegg-tenant-id` header on each request. See `resource_frontegg_tenant_sso_domain.go` for the pattern.

**Compound import IDs**: Some resources use colon-delimited IDs for import. Example: `tenant_sso_domain` uses `tenant_id:sso_config_id:domain_id`. Document import format in resource `Importer` block.

### Available Resources

- **Identity & Access**: roles, permissions, permission_category, users, portal_users, tenants
- **Authentication**: workspace, auth_policy, sso_domain_policy, social_login, auth0/cognito/firebase/custom_code user sources
- **Tenant SSO**: tenant_saml_config, tenant_oidc_config, tenant_sso_domain, tenant_sso_group_mapping, tenant_mfa_policy
- **Configuration**: admin_portal, email_template, email_provider, webhook, allowed_origin, redirect_uri, secret, prehook
- **Entitlements**: application, application_tenant_assignment, feature, plan, plan_feature

## Adding New Resources

1. Create `provider/resource_frontegg_<name>.go` with API struct + schema + CRUD functions
2. Register in `provider.go` `ResourcesMap`
3. Check `resource_frontegg_workspace.go` before defining new SSO-related structs — they may already exist there
4. Run `go generate` to update docs

## Debugging

```bash
export TF_LOG=TRACE  # REST client logs all HTTP requests/responses at TRACE level
terraform apply
```

## Testing

Acceptance tests (`TF_ACC=1`) create real Frontegg resources. Primary dev workflow is manual testing via `examples/basic/` using `make capply`.

## CI/CD

- **test.yml**: Acceptance tests on PR, push to master, and daily cron
- **release.yml**: goreleaser publishes provider on `v*` tag push
- **golangci-lint.yaml**: Lint checks
