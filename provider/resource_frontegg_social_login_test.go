package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func socialLoginResourceData(t *testing.T, raw map[string]interface{}) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, resourceFronteggSocialLogin().Schema, raw)
}

func TestSocialLoginDeserializeDropsCredentialsWhenNotCustomised(t *testing.T) {
	d := socialLoginResourceData(t, map[string]interface{}{})
	in := fronteggSSO{
		Active:      true,
		ClientID:    "shared-client-id",
		Secret:      "shared-secret",
		RedirectURL: "https://example.com/cb",
		Cusomised:   false,
	}

	if err := resourceFronteggSocialLoginDeserialize(d, in, "google"); err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if got := d.Get("client_id").(string); got != "" {
		t.Errorf("client_id = %q, want empty", got)
	}
	if got := d.Get("secret").(string); got != "" {
		t.Errorf("secret = %q, want empty", got)
	}
	if got := d.Get("redirect_url").(string); got != "https://example.com/cb" {
		t.Errorf("redirect_url = %q", got)
	}
}

func TestSocialLoginDeserializeKeepsCustomisedCredentials(t *testing.T) {
	d := socialLoginResourceData(t, map[string]interface{}{})
	in := fronteggSSO{
		Active:      true,
		ClientID:    "my-client-id",
		Secret:      "my-secret",
		RedirectURL: "https://example.com/cb",
		Cusomised:   true,
	}

	if err := resourceFronteggSocialLoginDeserialize(d, in, "google"); err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if got := d.Get("client_id").(string); got != "my-client-id" {
		t.Errorf("client_id = %q, want my-client-id", got)
	}
	if got := d.Get("secret").(string); got != "my-secret" {
		t.Errorf("secret = %q, want my-secret", got)
	}
}

const testAccSocialLogin = `
resource "frontegg_social_login" "test" {
  provider_name = "facebook"
  redirect_url  = "https://tf-acc.example.com/oauth/callback"
  customised    = false
}
`

func TestAccFronteggSocialLogin_lifecycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSocialLogin,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_social_login.test", "provider_name", "facebook"),
					resource.TestCheckResourceAttr("frontegg_social_login.test", "client_id", ""),
				),
			},
			{
				Config:   testAccSocialLogin,
				PlanOnly: true,
			},
			{
				ResourceName:      "frontegg_social_login.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "facebook",
			},
		},
	})
}

func TestSocialLoginAdoptionReason(t *testing.T) {
	tests := []struct {
		name               string
		existing           fronteggSSO
		configuredClientID string
		wantRefusal        bool
	}{
		{
			name:     "unconfigured provider is free to take",
			existing: fronteggSSO{Active: false, Cusomised: false},
		},
		{
			name:        "active provider is in use",
			existing:    fronteggSSO{Active: true},
			wantRefusal: true,
		},
		{
			name:        "active provider is in use even when we supply credentials",
			existing:    fronteggSSO{Active: true, ClientID: "theirs"},
			wantRefusal: true,
		},
		{
			name:        "inactive provider whose credentials we would erase",
			existing:    fronteggSSO{Active: false, ClientID: "theirs", Cusomised: true},
			wantRefusal: true,
		},
		{
			name:               "inactive provider whose credentials we are resupplying",
			existing:           fronteggSSO{Active: false, ClientID: "ours", Cusomised: true},
			configuredClientID: "ours",
		},
		{
			name:     "inactive provider holding only the shared credentials is free to take",
			existing: fronteggSSO{Active: false, ClientID: "shared", Cusomised: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := socialLoginAdoptionReason(tt.existing, tt.configuredClientID)
			if (got != "") != tt.wantRefusal {
				t.Errorf("reason = %q, wantRefusal = %v", got, tt.wantRefusal)
			}
		})
	}
}

const testAccSocialLoginAdopt = testAccSocialLogin + `
resource "frontegg_social_login" "adopt" {
  provider_name = "facebook"
  redirect_url  = "https://tf-acc-adopt.example.com/oauth/callback"
  customised    = false
}
`

func TestAccFronteggSocialLogin_refusesToAdopt(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{Config: testAccSocialLogin},
			{
				Config:      testAccSocialLoginAdopt,
				ExpectError: regexp.MustCompile(`cannot be created because it is already active`),
			},
		},
	})
}

func socialLoginFakeAPI(t *testing.T, failConfig bool) *httptest.Server {
	t.Helper()
	gets := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == fronteggSSOURL+"/facebook":
			gets++
			if gets > 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`{"type":"facebook","active":false,"customised":false}`))
		case r.Method == http.MethodPost && r.URL.Path == fronteggSSOURL:
			if failConfig {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == fronteggSSOURL+"/facebook/activate":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func socialLoginCreateAgainst(t *testing.T, srv *httptest.Server) (*schema.ResourceData, bool) {
	t.Helper()
	holder := &restclient.ClientHolder{ApiClient: restclient.MakeRestClient(srv.URL, "", "")}
	holder.ApiClient.Authenticate("test-token")
	d := socialLoginResourceData(t, map[string]interface{}{
		"provider_name": "facebook",
		"redirect_url":  "https://example.com/oauth/callback",
		"customised":    false,
	})
	diags := resourceFronteggSocialLoginCreate(context.Background(), d, holder)
	return d, diags.HasError()
}

func TestSocialLoginCreateKeepsIDWhenReadAfterActivationFails(t *testing.T) {
	d, failed := socialLoginCreateAgainst(t, socialLoginFakeAPI(t, false))
	if !failed {
		t.Fatal("create succeeded, want the failed read-back to surface as an error")
	}
	if d.Id() != "facebook" {
		t.Errorf("id = %q, want %q so the activated provider stays in state", d.Id(), "facebook")
	}
}

func TestSocialLoginCreateLeavesNoIDWhenConfigWriteFails(t *testing.T) {
	d, failed := socialLoginCreateAgainst(t, socialLoginFakeAPI(t, true))
	if !failed {
		t.Fatal("create succeeded, want the failed config write to surface as an error")
	}
	if d.Id() != "" {
		t.Errorf("id = %q, want empty because nothing was activated", d.Id())
	}
}
