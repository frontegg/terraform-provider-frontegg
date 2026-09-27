resource "frontegg_application" "example" {
  name           = "Example Application"
  app_url        = "https://app.example.com"
  login_url      = "https://app.example.com/login"
  logo_url       = "https://app.example.com/logo.png"
  # FREE_ACCESS (the default when omitted) assigns the application to every tenant
  # automatically. Use MANAGED_ACCESS to assign tenants explicitly instead, e.g. via
  # frontegg_application_tenant_assignment.
  access_type    = "FREE_ACCESS"
  is_default     = false
  is_active      = true
  type           = "web"
  frontend_stack = "react"
  description    = "An example application"

  metadata = {
    environment = "production"
    team        = "platform"
  }
}
