resource "frontegg_tenant_api_token" "example" {
  tenant_id   = "your-tenant-id"
  description = "CI/CD pipeline token"

  role_ids = [
    frontegg_role.admin.id,
  ]

  # Optional: token expires after 30 days (minimum 1 minute)
  expires_in_minutes = 43200

  # Optional: custom JWT metadata (must be a JSON object)
  metadata = jsonencode({
    service = "ci-pipeline"
    env     = "production"
  })
}

output "api_token_client_id" {
  value = frontegg_tenant_api_token.example.client_id
}

output "api_token_secret" {
  value     = frontegg_tenant_api_token.example.secret
  sensitive = true
}

# Import an existing token:
#   terraform import frontegg_tenant_api_token.example tenant-id:client-id
#
# Note: after import, `secret` and `metadata` will be empty in state.
# If `metadata` is set in config, a plan will show a diff and trigger replacement.
