package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

const computedValueSource = `
resource "frontegg_tenant" "source" {
  key  = "computed-value-source"
  name = "computed value source"
}
`

func testPlanOnly(t *testing.T, config string, expectError *regexp.Regexp) {
	t.Helper()
	newFakeAPI(t)
	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{{
			Config:             config,
			PlanOnly:           true,
			ExpectNonEmptyPlan: expectError == nil,
			ExpectError:        expectError,
		}},
	})
}

func TestRequiredFieldsAcceptValuesComputedAtApply(t *testing.T) {
	testPlanOnly(t, computedValueSource+`
resource "frontegg_prehook" "test" {
  enabled     = true
  name        = "computed url"
  description = "computed url"
  events      = ["USER_INVITE"]
  fail_method = "OPEN"
  url         = "https://example.com/${frontegg_tenant.source.id}"
  secret      = frontegg_tenant.source.id
}

resource "frontegg_social_login" "test" {
  provider_name = "google"
  redirect_url  = "https://example.com/callback"
  customised    = true
  client_id     = frontegg_tenant.source.id
  secret        = frontegg_tenant.source.id
}

resource "frontegg_email_provider" "test" {
  provider_name = "mailgun"
  secret        = "mailgun-secret"
  domain        = frontegg_tenant.source.id
  region        = frontegg_tenant.source.id
}

resource "frontegg_federation_user_source" "test" {
  name                 = "computed secret"
  sync_on_login        = false
  tenant_resolver_type = "new"
  client_id            = "federation-client"
  wellknown_url        = "https://idp.example.com/.well-known/openid-configuration"
  secret               = frontegg_tenant.source.id
}
`, nil)
}

func TestSocialLoginRequiresClientIDWhenCustomised(t *testing.T) {
	testPlanOnly(t, `
resource "frontegg_social_login" "test" {
  provider_name = "google"
  redirect_url  = "https://example.com/callback"
  customised    = true
  secret        = "google-secret"
}
`, regexp.MustCompile("client_id is required when customised is true"))
}

func TestFederationUserSourceRequiresAnEndpointSource(t *testing.T) {
	testPlanOnly(t, `
resource "frontegg_federation_user_source" "test" {
  name                 = "no endpoints"
  sync_on_login        = false
  tenant_resolver_type = "new"
  client_id            = "federation-client"
  secret               = "federation-secret"
}
`, regexp.MustCompile("one of\\s+`oauth2_config,wellknown_url`"))
}

func TestFederationUserSourceRequiresSecretWithoutPKCE(t *testing.T) {
	testPlanOnly(t, `
resource "frontegg_federation_user_source" "test" {
  name                 = "no secret"
  sync_on_login        = false
  tenant_resolver_type = "new"
  client_id            = "federation-client"
  wellknown_url        = "https://idp.example.com/.well-known/openid-configuration"
}
`, regexp.MustCompile("secret or secret_wo is required when use_pkce is false"))
}

func TestFederationUserSourceWithPKCEDoesNotNeedSecret(t *testing.T) {
	testPlanOnly(t, `
resource "frontegg_federation_user_source" "test" {
  name                 = "pkce"
  sync_on_login        = false
  tenant_resolver_type = "new"
  client_id            = "federation-client"
  wellknown_url        = "https://idp.example.com/.well-known/openid-configuration"
  use_pkce             = true
}
`, nil)
}
