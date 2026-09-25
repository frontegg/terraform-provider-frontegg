package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
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

func TestProviderRefreshesTokenForBothClients(t *testing.T) {
	var issued int
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/vendor" {
			issued++
			// Only the first token expires quickly, so the refreshed token
			// cannot also expire between the API and portal requests.
			expiresIn := 3600.0
			if issued == 1 {
				expiresIn = 0.001
			}
			_, _ = fmt.Fprintf(w, `{"token":"token-%d","expiresIn":%g}`, issued, expiresIn)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-2" {
			t.Errorf("API Authorization = %q, want refreshed token", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer api.Close()
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token-2" {
			t.Errorf("portal Authorization = %q, want refreshed token", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer portal.Close()

	provider := New("test")()
	d := schema.TestResourceDataRaw(t, provider.Schema, map[string]interface{}{
		"api_base_url": api.URL, "portal_base_url": portal.URL,
		"client_id": "client", "secret_key": "secret",
	})
	meta, diags := provider.ConfigureContextFunc(context.Background(), d)
	if diags.HasError() {
		t.Fatalf("ConfigureContextFunc: %v", diags)
	}
	// The first token's short lifetime should expire before these calls.
	// Both clients must then use the same refreshed token.
	<-time.After(5 * time.Millisecond)
	holder := meta.(*restclient.ClientHolder)
	if err := holder.ApiClient.Get(context.Background(), "/api", nil); err != nil {
		t.Fatal(err)
	}
	if err := holder.PortalClient.Get(context.Background(), "/portal", nil); err != nil {
		t.Fatal(err)
	}
	if issued != 2 {
		t.Fatalf("vendor tokens issued = %d, want 2", issued)
	}
}
