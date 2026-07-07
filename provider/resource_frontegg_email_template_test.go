package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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
