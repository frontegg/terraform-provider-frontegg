package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var testAccProviderFactories = map[string]func() (*schema.Provider, error){
	"frontegg": func() (*schema.Provider, error) {
		return New("test")(), nil
	},
}

func testAccPreCheck(t *testing.T) {
	for _, key := range []string{"FRONTEGG_CLIENT_ID", "FRONTEGG_SECRET_KEY"} {
		if os.Getenv(key) == "" {
			t.Skipf("%s must be set for acceptance tests", key)
		}
	}
}

func TestValidateProviderSchema(t *testing.T) {
	scm := schema.InternalMap(New("0.0.0")().Schema)
	if err := scm.InternalValidate(nil); err != nil {
		t.Errorf("Schema failed to validate: %v", err)
	}
}

func TestValidateResourceSchemas(t *testing.T) {
	prov := New("0.0.0")()
	for name, res := range prov.ResourcesMap {
		test := res
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			scm := schema.InternalMap(test.Schema)
			if err := scm.InternalValidate(nil); err != nil {
				t.Errorf("Schema failed to validate: %v", err)
			}
		})
	}
}

func TestSecretAttributesAreSensitive(t *testing.T) {
	for name, res := range New("0.0.0")().ResourcesMap {
		assertSecretsSensitive(t, name, res.Schema)
	}
}

func assertSecretsSensitive(t *testing.T, path string, attrs map[string]*schema.Schema) {
	t.Helper()
	for key, attr := range attrs {
		if key == "secret" && !attr.Sensitive {
			t.Errorf("%s.%s must be Sensitive", path, key)
		}
		if nested, ok := attr.Elem.(*schema.Resource); ok {
			assertSecretsSensitive(t, path+"."+key, nested.Schema)
		}
	}
}

func TestBaseURLsHonourEnvironment(t *testing.T) {
	t.Setenv("FRONTEGG_API_BASE_URL", "https://api.us.frontegg.com")
	t.Setenv("FRONTEGG_PORTAL_BASE_URL", "https://frontegg-prod.us.frontegg.com")

	d := schema.TestResourceDataRaw(t, New("0.0.0")().Schema, map[string]interface{}{})

	if got := d.Get("api_base_url"); got != "https://api.us.frontegg.com" {
		t.Errorf("api_base_url = %q, want the FRONTEGG_API_BASE_URL value", got)
	}
	if got := d.Get("portal_base_url"); got != "https://frontegg-prod.us.frontegg.com" {
		t.Errorf("portal_base_url = %q, want the FRONTEGG_PORTAL_BASE_URL value", got)
	}
}

func TestBaseURLsFallBackToEU(t *testing.T) {
	t.Setenv("FRONTEGG_API_BASE_URL", "")
	t.Setenv("FRONTEGG_PORTAL_BASE_URL", "")

	d := schema.TestResourceDataRaw(t, New("0.0.0")().Schema, map[string]interface{}{})

	if got := d.Get("api_base_url"); got != "https://api.frontegg.com" {
		t.Errorf("api_base_url = %q, want https://api.frontegg.com", got)
	}
	if got := d.Get("portal_base_url"); got != "https://frontegg-prod.frontegg.com" {
		t.Errorf("portal_base_url = %q, want https://frontegg-prod.frontegg.com", got)
	}
}
