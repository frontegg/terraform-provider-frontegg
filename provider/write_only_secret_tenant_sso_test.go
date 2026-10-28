package provider

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func fakeTenantSSOAPI(t *testing.T) *fakeAPI {
	t.Helper()
	return newFakeAPI(t,
		fakeAPIRoute{http.MethodPost, fronteggTenantSSOConfigV2Path, fakeCreate("id")},
		fakeAPIRoute{http.MethodGet, fronteggTenantSSOConfigPath, fakeList},
		fakeAPIRoute{http.MethodPatch, fronteggTenantSSOConfigV2Path + "/{id}", fakeUpdate},
		fakeAPIRoute{http.MethodDelete, fronteggTenantSSOConfigPath + "/{id}", fakeDelete},
	)
}

func tenantSSOImportID(address string) func(*terraform.State) (string, error) {
	return func(s *terraform.State) (string, error) {
		return "tenant-1:" + s.RootModule().Resources[address].Primary.ID, nil
	}
}

func TestTenantOIDCConfigWriteOnlySecretLifecycle(t *testing.T) {
	api := fakeTenantSSOAPI(t)
	testWriteOnlySecretLifecycle(t, api, writeOnlySecretCase{
		address:  "frontegg_tenant_oidc_config.test",
		secrets:  map[string][]string{"oidc_secret": {"oidcSecret"}},
		importID: tenantSSOImportID("frontegg_tenant_oidc_config.test"),
		config: func(arguments string) string {
			return `
resource "frontegg_tenant_oidc_config" "test" {
  tenant_id      = "tenant-1"
  enabled        = false
  sso_endpoint   = "https://idp.example.com"
  oidc_client_id = "oidc-client"
` + arguments + `}
`
		},
	})
}

func TestTenantSAMLConfigWriteOnlySecretLifecycle(t *testing.T) {
	api := fakeTenantSSOAPI(t)
	testWriteOnlySecretLifecycle(t, api, writeOnlySecretCase{
		address:  "frontegg_tenant_saml_config.test",
		secrets:  map[string][]string{"idp_client_secret": {"idpClientSecret"}},
		importID: tenantSSOImportID("frontegg_tenant_saml_config.test"),
		config: func(arguments string) string {
			return `
resource "frontegg_tenant_saml_config" "test" {
  tenant_id     = "tenant-1"
  enabled       = false
  sso_endpoint  = "https://idp.example.com/saml"
  idp_client_id = "saml-client"
` + arguments + `}
`
		},
	})
}
