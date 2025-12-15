# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a Terraform provider for the Frontegg user management platform, written in Go. The provider enables infrastructure-as-code management of Frontegg resources including workspaces, roles, permissions, authentication policies, and more.

## Requirements

- **Terraform** >= 1.0.3
- **Go** >= 1.20 (pay attention to install platform compatible version)
- Environment variables for authentication:
  - `FRONTEGG_CLIENT_ID` - Your Frontegg API client ID
  - `FRONTEGG_SECRET_KEY` - Your Frontegg API secret key

## Common Commands

### Building and Installing

```bash
# Build and install provider locally to ~/.terraform.d/plugins
make install

# Generate documentation (uses terraform-plugin-docs)
go generate

# Install and apply changes (after provider code changes)
make capply    # Runs: make install && terraform init && terraform apply
```

### Testing

```bash
# Run acceptance tests (creates real resources, may cost money)
make testacc   # Equivalent to: TF_ACC=1 go test ./... -v -timeout 120m

# Run tests for specific package
go test -v -cover ./provider/

# Run tests with debugging logs
TF_LOG=DEBUG make testacc
```

### Linting and Code Quality

```bash
# Run all linting checks
make lint      # Runs: go vet, gofmt check, go mod tidy

# Auto-fix linting issues
make lint-fix  # Runs: go vet, gofmt -w, go mod tidy
```

### Development Iteration Cycle

When manually testing changes (recommended workflow):

```bash
# Set authentication environment variables
export FRONTEGG_CLIENT_ID=<your-client-id>
export FRONTEGG_SECRET_KEY=<your-secret-key>

# Navigate to examples directory
cd examples/basic

# Recompile, reinitialize, and apply in one command
(cd ../.. && make install) && rm .terraform.lock.hcl && terraform init && terraform apply -auto-approve
```

### Terraform Operations

```bash
# Initialize Terraform with plugin upgrades
terraform init -upgrade

# Plan changes
terraform plan

# Apply changes
terraform apply -auto-approve

# Destroy resources
terraform destroy -auto-approve
```

## Architecture

### Project Structure

```
.
├── main.go                    # Provider entry point
├── provider/                  # Provider implementation
│   ├── provider.go           # Provider configuration and registration
│   ├── resource_frontegg_*.go # Individual resource implementations
│   ├── data_source_*.go      # Data source implementations
│   └── validators/           # Custom validators
├── internal/
│   └── restclient/           # HTTP client for Frontegg API
│       ├── restclient.go     # REST client implementation
│       └── clientholder.go   # Holds API and Portal clients
├── examples/                  # Example Terraform configurations
│   ├── basic/                # Basic usage example
│   ├── resources/            # Resource-specific examples
│   └── data-sources/         # Data source examples
├── docs/                      # Auto-generated documentation
└── templates/                 # Documentation templates
```

### Provider Architecture

The provider follows the standard Terraform plugin SDK v2 architecture:

1. **main.go**: Entry point that initializes the provider using `plugin.Serve()`
2. **provider.go**: Defines provider configuration, resources, and data sources
3. **Resource Files**: Each Frontegg resource type (e.g., roles, permissions, workspaces) has its own file implementing CRUD operations
4. **REST Client**: A custom HTTP client (`internal/restclient`) handles authentication and API communication

### Key Concepts

#### Provider Configuration

The provider requires authentication credentials and supports regional API endpoints:
- `client_id` / `FRONTEGG_CLIENT_ID`: API client ID
- `secret_key` / `FRONTEGG_SECRET_KEY`: API secret key
- `api_base_url` / `FRONTEGG_API_BASE_URL`: API endpoint (default: EU region)
- `portal_base_url` / `FRONTEGG_PORTAL_BASE_URL`: Portal endpoint (default: EU region)
- `environment_id`: Optional environment-specific ID

#### Resource Implementation Pattern

Each resource follows this standard pattern:
1. Define a Go struct representing the API model
2. Define a `schema.Resource` with CRUD operations
3. Implement Create/Read/Update/Delete context functions
4. Use the REST client to communicate with Frontegg APIs
5. Handle state management through Terraform's ResourceData

#### Dual Client Architecture

The provider uses two REST clients:
- **API Client**: For most resource operations (uses `api.frontegg.com`)
- **Portal Client**: For portal-specific operations (uses `frontegg-prod.frontegg.com`)

Both clients authenticate using the same token obtained via `/auth/vendor` endpoint.

### Available Resources

The provider manages these Frontegg resources:
- **Identity & Access**: roles, permissions, permission_category, users, portal_users, tenants
- **Authentication**: workspace, auth_policy, sso_domain_policy, social_login, auth0/cognito/firebase/custom_code user sources
- **Configuration**: admin_portal, email_template, email_provider, webhook, allowed_origin, redirect_uri, secret, prehook
- **Entitlements**: application, application_tenant_assignment, feature, plan, plan_feature

## Development Guidelines

### Adding New Resources

1. Create a new file `provider/resource_frontegg_<resource_name>.go`
2. Define the API model struct with JSON tags
3. Implement the resource schema with proper descriptions
4. Implement CRUD functions using the REST client pattern
5. Register the resource in `provider.go` ResourcesMap
6. Add examples in `examples/resources/<resource_name>/`
7. Run `go generate` to update documentation

### Adding Dependencies

This provider uses Go modules:

```bash
go get github.com/author/dependency
go mod tidy
git commit go.mod go.sum
```

### Debugging

Enable detailed Terraform logs:

```bash
export TF_LOG=TRACE  # or DEBUG, INFO, WARN, ERROR
terraform apply
```

The REST client logs all HTTP requests/responses when trace logging is enabled.

### Importing Existing Resources

Resources can be imported using their IDs:

```bash
# Add shim resource definition first
terraform import frontegg_workspace.example <workspace-id>

# View imported state
terraform state show frontegg_workspace.example

# Copy output to your .tf file and verify
terraform plan  # Should show no changes
```

For roles, permissions, and permission categories, find IDs via:
- The Frontegg API directly
- Browser dev tools while using the Frontegg Portal
- The `frontegg_permission` data source (for permissions)

## Testing Strategy

### Acceptance Tests

- Run with `TF_ACC=1` environment variable
- Create real resources in Frontegg (may incur costs)
- Require valid `FRONTEGG_CLIENT_ID` and `FRONTEGG_SECRET_KEY`
- Located in `provider/` with `_test.go` suffix
- Run daily via GitHub Actions cron schedule

### Manual Testing

Manual testing is the primary development workflow:
1. Make code changes
2. Run `make install` to rebuild and install locally
3. Use `examples/basic/` to test changes
4. Remove `.terraform.lock.hcl` to force provider refresh
5. Run `terraform apply` to test

## CI/CD

### GitHub Actions Workflows

- **test.yml**: Runs acceptance tests on PR, push to master, and daily schedule
- **release.yml**: Uses goreleaser to publish provider releases
- **golangci-lint.yaml**: Runs golangci-lint checks

### Release Process

Releases are automated via goreleaser when tags are pushed:
- Version is injected from git tags (format: `v*`)
- Provider binaries are built for multiple platforms
- Published to Terraform Registry

## API Communication

The REST client (`internal/restclient`) provides:
- Standard HTTP methods: GET, POST, PUT, PATCH, DELETE
- Automatic Bearer token authentication
- Environment ID header injection
- Conflict retry mechanism (409 responses)
- 404 ignore option for delete operations
- JSON serialization/deserialization
- Detailed trace logging
