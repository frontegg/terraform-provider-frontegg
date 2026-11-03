package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestSAMLConfigDeserializeKeepsIdPClientWhenAPIOmitsIt(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceFronteggTenantSAMLConfig().Schema, map[string]interface{}{
		"tenant_id":         "tenant-1",
		"sso_endpoint":      "https://idp.example.com/saml",
		"idp_client_id":     "entra-app",
		"idp_client_secret": "entra-secret",
	})

	err := samlConfigDeserialize(d, fronteggTenantSSOConfig{
		ID:          "config-1",
		SSOEndpoint: "https://idp.example.com/saml",
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := d.Get("idp_client_id"); got != "entra-app" {
		t.Errorf("idp_client_id = %q, want entra-app", got)
	}
	if got := d.Get("idp_client_secret"); got != "entra-secret" {
		t.Errorf("idp_client_secret = %q, want entra-secret", got)
	}
}

func TestSAMLConfigDeserializeRefreshesIdPClientWhenAPIReturnsIt(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceFronteggTenantSAMLConfig().Schema, map[string]interface{}{
		"tenant_id":         "tenant-1",
		"idp_client_id":     "entra-app",
		"idp_client_secret": "entra-secret",
	})

	err := samlConfigDeserialize(d, fronteggTenantSSOConfig{
		ID:              "config-1",
		IDPClientID:     "changed-app",
		IDPClientSecret: "changed-secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := d.Get("idp_client_id"); got != "changed-app" {
		t.Errorf("idp_client_id = %q, want changed-app", got)
	}
	if got := d.Get("idp_client_secret"); got != "changed-secret" {
		t.Errorf("idp_client_secret = %q, want changed-secret", got)
	}
}
