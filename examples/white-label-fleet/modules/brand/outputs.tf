output "tenant_id" {
  description = "The tenant created for this brand."
  value       = frontegg_tenant.this.id
}

output "application_id" {
  description = "The application created for this brand."
  value       = frontegg_application.this.id
}

output "auth_host" {
  description = "The brand's authentication host, to be aggregated into the workspace custom_domains set."
  value       = var.auth_host
}

output "redirect_uris" {
  description = "Every redirect URI registered for this brand."
  value       = local.redirect_uris
}
