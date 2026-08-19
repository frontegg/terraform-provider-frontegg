package provider

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// TestEmailTemplateDeserializePrefersPatternFields verifies that reading a
// template prefers the raw redirectURLPattern/successRedirectUrlPattern fields
// over the rendered redirectURL/successRedirectUrl fields, so configurations
// using template variables (e.g. "{{APP_ID}}") don't produce a permanent diff.
func TestEmailTemplateDeserializePrefersPatternFields(t *testing.T) {
	tests := []struct {
		name                   string
		in                     fronteggEmailTemplateResource
		wantRedirectURL        string
		wantSuccessRedirectURL string
	}{
		{
			name: "pattern fields present take precedence over rendered values",
			in: fronteggEmailTemplateResource{
				Type:                      "ActivateUser",
				RedirectURL:               "https://example.com/activate?appId=",
				RedirectURLPattern:        "https://example.com/activate?appId={{APP_ID}}",
				SuccessRedirectURL:        "http://localhost:3000/",
				SuccessRedirectURLPattern: "{{APP_URL}}",
			},
			wantRedirectURL:        "https://example.com/activate?appId={{APP_ID}}",
			wantSuccessRedirectURL: "{{APP_URL}}",
		},
		{
			name: "rendered values used when pattern fields are absent",
			in: fronteggEmailTemplateResource{
				Type:               "ResetPassword",
				RedirectURL:        "https://example.com/reset-password",
				SuccessRedirectURL: "https://example.com/done",
			},
			wantRedirectURL:        "https://example.com/reset-password",
			wantSuccessRedirectURL: "https://example.com/done",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := schema.TestResourceDataRaw(t, resourceFronteggEmailTemplate().Schema, map[string]interface{}{})
			if err := resourceFronteggEmailTemplateDeserialize(d, tt.in); err != nil {
				t.Fatalf("resourceFronteggEmailTemplateDeserialize() error = %v", err)
			}
			if got := d.Get("redirect_url").(string); got != tt.wantRedirectURL {
				t.Errorf("redirect_url = %q, want %q", got, tt.wantRedirectURL)
			}
			if got := d.Get("success_redirect_url").(string); got != tt.wantSuccessRedirectURL {
				t.Errorf("success_redirect_url = %q, want %q", got, tt.wantSuccessRedirectURL)
			}
		})
	}
}

// Email templates cannot be deleted — delete only drops them from state — so a
// normal acceptance test would permanently alter a shared template. This drives
// the read path with only the ID set, exactly as an import does, and mutates
// nothing.
func TestAccEmailTemplateImportRead(t *testing.T) {
	testAccPreCheck(t)

	p := New("test")()
	cfg := terraform.NewResourceConfigRaw(map[string]interface{}{
		"client_id":  os.Getenv("FRONTEGG_CLIENT_ID"),
		"secret_key": os.Getenv("FRONTEGG_SECRET_KEY"),
	})
	if diags := p.Configure(context.Background(), cfg); diags.HasError() {
		t.Fatalf("configure provider: %v", diags)
	}

	d := schema.TestResourceDataRaw(t, resourceFronteggEmailTemplate().Schema, map[string]interface{}{})
	d.SetId("ResetPassword")

	if diags := resourceFronteggEmailTemplateRead(context.Background(), d, p.Meta()); diags.HasError() {
		t.Fatalf("read: %v", diags)
	}
	if d.Id() == "" {
		t.Fatal("read cleared the ID, so import would fail")
	}
	if got := d.Get("template_type").(string); got != "ResetPassword" {
		t.Errorf("template_type = %q, want ResetPassword", got)
	}
}
