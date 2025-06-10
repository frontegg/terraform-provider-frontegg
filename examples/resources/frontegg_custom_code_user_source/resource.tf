resource "frontegg_custom_code_user_source" "example" {
  name                 = "Example Custom Code User Source"
  description          = "An example custom code user source"
  index                = 1
  sync_on_login        = true
  is_migrated          = false
  tenant_resolver_type = "static"
  tenant_id            = "tenant-1234567890"

  code_payload = <<-EOT
    function authenticate(email, password) {
      // Custom authentication logic
      if (email === 'user@example.com' && password === 'password123') {
        return {
          id: 'user-123',
          email: 'user@example.com',
          name: 'Example User'
        };
      }
      return null;
    }
  EOT

  get_user_code_payload = <<-EOT
    function getUser(userId) {
      // Custom user retrieval logic
      if (userId === 'user-123') {
        return {
          id: 'user-123',
          email: 'user@example.com',
          name: 'Example User',
          metadata: {
            role: 'admin'
          }
        };
      }
      return null;
    }
  EOT

  app_ids = [
    "app-1234567890"
  ]
}
