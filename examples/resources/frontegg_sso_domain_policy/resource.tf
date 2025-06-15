# This resource configures how SSO domains are validated
# This is a singleton resource - only one per workspace
resource "frontegg_sso_domain_policy" "example" {
  allow_verified_users_to_add_domains = false
  skip_domain_verification            = false
  bypass_domain_cross_validation      = false
}
