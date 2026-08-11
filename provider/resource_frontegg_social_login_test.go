package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func socialLoginResourceData(t *testing.T, raw map[string]interface{}) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, resourceFronteggSocialLogin().Schema, raw)
}

// A provider that is not using customised credentials still has credentials on
// the API side. Copying those into state would fight a configuration that
// deliberately omits them.
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

// facebook is deliberately chosen: it is the provider least likely to already
// be configured, so the test does not overwrite live credentials.
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
			// A customised resource re-applied after destroy: inactive, and the
			// configuration supplies the client ID again. Must not refuse.
			name:               "inactive provider whose credentials we are resupplying",
			existing:           fronteggSSO{Active: false, ClientID: "ours", Cusomised: true},
			configuredClientID: "ours",
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

// The provider this contends with is the one the test just created, so the
// refusal is exercised without putting a pre-existing configuration at risk.
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
