resource "frontegg_application" "example" {
  name           = "Example Application"
  app_url        = "https://app.example.com"
  login_url      = "https://app.example.com/login"
  logo_url       = "https://app.example.com/logo.png"
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
