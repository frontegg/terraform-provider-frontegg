---
layout: ""
page_title: "Frontegg Provider v2.0.0 Migration Guide"
description: |-
  Migration guide for upgrading from Frontegg Provider v1.0.x to v2.0.0, covering breaking changes and step-by-step migration instructions.
---

# Frontegg Terraform Provider v2.0.0 Migration Guide

## Overview

Starting with version 1.1.0 of the Frontegg Terraform Provider, the `frontegg_workspace` resource has been split into multiple smaller, more focused resources. This change improves modularity, makes configurations more maintainable, and allows for better resource management.

## Breaking Changes Summary

The following fields have been **removed** from the `frontegg_workspace` resource and moved to dedicated resources:

1. **Authentication Policy** → `frontegg_auth_policy`
2. **Admin Portal Configuration** → `frontegg_admin_portal`
3. **Email Templates** → `frontegg_email_template`
4. **Social Login Providers** → `frontegg_social_login`
5. **SSO Domain Policy** → `frontegg_sso_domain_policy`

## Example Files

Complete example configurations for all new resources are available in the `examples/resources/` directory:

- `examples/resources/frontegg_auth_policy/resource.tf`
- `examples/resources/frontegg_admin_portal/resource.tf`
- `examples/resources/frontegg_email_template/resource.tf`
- `examples/resources/frontegg_social_login/resource.tf`
- `examples/resources/frontegg_sso_domain_policy/resource.tf`
- `examples/resources/frontegg_workspace/resource.tf` (updated)

## Migration Steps

### 1. Authentication Policy Migration

**Before (v1.0.x):**
```hcl
resource "frontegg_workspace" "my_workspace" {
  # ... other fields ...
  
  auth_policy {
    allow_unverified_users              = false
    allow_signups                      = true
    allow_tenant_invitations           = true
    enable_api_tokens                  = true
    machine_to_machine_auth_strategy   = "ClientCredentials"
    enable_roles                       = true
    jwt_algorithm                      = "RS256"
    jwt_access_token_expiration        = 3600
    jwt_refresh_token_expiration       = 2592000
    same_site_cookie_policy            = "lax"
    auth_strategy                      = "EmailAndPassword"
  }
}
```

**After (v1.1.0+):**
```hcl
resource "frontegg_workspace" "my_workspace" {
  # ... other fields (auth_policy block removed) ...
}

resource "frontegg_auth_policy" "my_auth_policy" {
  allow_unverified_users              = false
  allow_signups                      = true
  allow_tenant_invitations           = true
  enable_api_tokens                  = true
  machine_to_machine_auth_strategy   = "ClientCredentials"
  enable_roles                       = true
  jwt_algorithm                      = "RS256"
  jwt_access_token_expiration        = 3600
  jwt_refresh_token_expiration       = 2592000
  same_site_cookie_policy            = "lax"
  auth_strategy                      = "EmailAndPassword"
}
```

### 2. Admin Portal Configuration Migration

**Before (v1.0.x):**
```hcl
resource "frontegg_workspace" "my_workspace" {
  # ... other fields ...
  
  admin_portal {
    enable_account_settings    = true
    enable_api_tokens         = true
    enable_audit_logs         = true
    enable_groups            = true
    enable_personal_api_tokens = true
    enable_privacy           = true
    enable_profile           = true
    enable_provisioning      = true
    enable_roles             = true
    enable_security          = true
    enable_sso               = true
    enable_subscriptions     = true
    enable_usage             = true
    enable_users             = true
    enable_webhooks          = true
    
    palette {
      primary {
        main = "#1976d2"
        light = "#42a5f5"
        dark = "#1565c0"
        contrast_text = "#ffffff"
        active = "#1565c0"
        hover = "#1976d2"
      }
    }
  }
}
```

**After (v1.1.0+):**
```hcl
resource "frontegg_workspace" "my_workspace" {
  # ... other fields (admin_portal block removed) ...
}

resource "frontegg_admin_portal" "my_admin_portal" {
  enable_account_settings    = true
  enable_api_tokens         = true
  enable_audit_logs         = true
  enable_groups            = true
  enable_personal_api_tokens = true
  enable_privacy           = true
  enable_profile           = true
  enable_provisioning      = true
  enable_roles             = true
  enable_security          = true
  enable_sso               = true
  enable_subscriptions     = true
  enable_usage             = true
  enable_users             = true
  enable_webhooks          = true
  
  palette {
    primary {
      main = "#1976d2"
      light = "#42a5f5"
      dark = "#1565c0"
      contrast_text = "#ffffff"
      active = "#1565c0"
      hover = "#1976d2"
    }
  }
}
```

### 3. Email Templates Migration

**Before (v1.0.x):**
```hcl
resource "frontegg_workspace" "my_workspace" {
  # ... other fields ...
  
  email_template {
    type                = "UserActivation"
    active              = true
    from_name           = "My App"
    html_template       = "<html>...</html>"
    redirect_url        = "https://myapp.com/activate"
    sender_email        = "noreply@myapp.com"
    subject             = "Activate your account"
    success_redirect_url = "https://myapp.com/activated"
  }
}
```

**After (v1.1.0+):**
```hcl
resource "frontegg_workspace" "my_workspace" {
  # ... other fields (email_template blocks removed) ...
}

resource "frontegg_email_template" "user_activation" {
  type                = "UserActivation"
  active              = true
  from_name           = "My App"
  html_template       = "<html>...</html>"
  redirect_url        = "https://myapp.com/activate"
  sender_email        = "noreply@myapp.com"
  subject             = "Activate your account"
  success_redirect_url = "https://myapp.com/activated"
}
```

### 4. Social Login Migration

**Before (v1.0.x):**
```hcl
resource "frontegg_workspace" "my_workspace" {
  # ... other fields ...
  
  google {
    client_id     = "your-google-client-id"
    redirect_url  = "https://myapp.com/auth/google/callback"
    secret        = "your-google-secret"
    customised    = true
  }
  
  github {
    client_id     = "your-github-client-id"
    redirect_url  = "https://myapp.com/auth/github/callback"
    secret        = "your-github-secret"
    customised    = true
  }
}
```

**After (v1.1.0+):**
```hcl
resource "frontegg_workspace" "my_workspace" {
  # ... other fields (social login blocks removed) ...
}

resource "frontegg_social_login" "google" {
  provider_name = "google"
  client_id     = "your-google-client-id"
  redirect_url  = "https://myapp.com/auth/google/callback"
  secret        = "your-google-secret"
  customised    = true
}

resource "frontegg_social_login" "github" {
  provider_name = "github"
  client_id     = "your-github-client-id"
  redirect_url  = "https://myapp.com/auth/github/callback"
  secret        = "your-github-secret"
  customised    = true
}
```

### 5. SSO Domain Policy Migration

**Before (v1.0.x):**
```hcl
resource "frontegg_workspace" "my_workspace" {
  # ... other fields ...
  
  sso_domain_policy {
    allow_verified_users_to_add_domains = false
    skip_domain_verification            = false
    bypass_domain_cross_validation      = false
  }
}
```

**After (v1.1.0+):**
```hcl
resource "frontegg_workspace" "my_workspace" {
  # ... other fields (sso_domain_policy block removed) ...
}

resource "frontegg_sso_domain_policy" "my_sso_domain_policy" {
  allow_verified_users_to_add_domains = false
  skip_domain_verification            = false
  bypass_domain_cross_validation      = false
}
```

## What Remains in `frontegg_workspace`

The following fields **remain** in the `frontegg_workspace` resource:

- `name` - Workspace name
- `country` - Associated country
- `backend_stack` - Backend technology stack
- `frontend_stack` - Frontend technology stack
- `open_saas_installed` - OpenSaaS installation status
- `frontegg_domain` - Frontegg domain
- `custom_domains` - Custom domains
- `allowed_origins` - Allowed origins
- `mfa_policy` - MFA policy configuration
- `mfa_authentication_app` - MFA authentication app settings
- `lockout_policy` - User lockout policy
- `password_policy` - Password policy
- `captcha_policy` - CAPTCHA policy
- `hosted_login` - Hosted login configuration
- `saml` - SAML SSO configuration
- `oidc` - OIDC SSO configuration
- `sso_multi_tenant_policy` - Multi-tenant SSO policy

## Step-by-Step Migration Process

1. **Backup your current state:**
   ```bash
   terraform state pull > backup.tfstate
   ```

2. **Update your Terraform configuration** using the examples above.

3. **Import existing resources** (if they exist in your Frontegg workspace):
   ```bash
   # Auth policy is a singleton, use a fixed ID
   terraform import frontegg_auth_policy.my_auth_policy auth_policy
   
   # Admin portal is a singleton, use a fixed ID
   terraform import frontegg_admin_portal.my_admin_portal admin_portal
   
   # SSO domain policy is a singleton, use a fixed ID
   terraform import frontegg_sso_domain_policy.my_sso_domain_policy sso_domain_policy
   
   # Email templates use their type as ID
   terraform import frontegg_email_template.user_activation UserActivation
   
   # Social logins use provider name as ID
   terraform import frontegg_social_login.google google
   terraform import frontegg_social_login.github github
   ```

4. **Plan and apply:**
   ```bash
   terraform plan
   terraform apply
   ```

## Important Notes

- **Singleton Resources**: `frontegg_auth_policy`, `frontegg_admin_portal`, and `frontegg_sso_domain_policy` are singleton resources (only one per workspace).
- **Resource IDs**: Use the import commands above to properly link existing configurations.
- **State Management**: The provider will handle backward compatibility for reading existing configurations.
- **No Data Loss**: All existing configurations will be preserved during migration.

## Troubleshooting

If you encounter issues during migration:

1. **State conflicts**: Use `terraform state rm` to remove old resource references if needed
2. **Import errors**: Ensure you're using the correct resource IDs as shown above
3. **Configuration drift**: Run `terraform plan` to identify any configuration differences

## Benefits of the New Structure

- **Modular Configuration**: Each aspect of your Frontegg setup is now independently manageable
- **Better Resource Lifecycle Management**: Updates to one component don't affect others
- **Improved Maintainability**: Smaller, focused resources are easier to understand and maintain
- **Enhanced Reusability**: Resource configurations can be more easily shared across environments 