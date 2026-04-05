package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceFronteggTenantOIDCConfig() *schema.Resource {
	s := commonSSOSchema()

	s["oidc_client_id"] = &schema.Schema{
		Description: "The client ID of the OIDC application registered on the external IdP (e.g. Okta, Azure AD).",
		Type:        schema.TypeString,
		Optional:    true,
	}
	s["oidc_secret"] = &schema.Schema{
		Description: "The client secret for the OIDC application. Used with `oidc_client_id` to authenticate token exchange requests with the IdP.",
		Type:        schema.TypeString,
		Optional:    true,
		Sensitive:   true,
	}

	return &schema.Resource{
		Description: `Configures an OIDC SSO configuration for a Frontegg tenant. Users whose email domain matches a domain associated with this configuration will be redirected to the OIDC Identity Provider (IdP) for authentication.`,

		CreateContext: resourceFronteggTenantOIDCConfigCreate,
		ReadContext:   resourceFronteggTenantOIDCConfigRead,
		UpdateContext: resourceFronteggTenantOIDCConfigUpdate,
		DeleteContext: tenantSSODelete,
		Importer: &schema.ResourceImporter{
			StateContext: tenantSSOImport,
		},
		Schema: s,
	}
}

func oidcConfigSerialize(d *schema.ResourceData) fronteggTenantSSOConfig {
	return fronteggTenantSSOConfig{
		Type:                      "oidc",
		Enabled:                   d.Get("enabled").(bool),
		SSOEndpoint:               d.Get("sso_endpoint").(string),
		OIDCClientID:              d.Get("oidc_client_id").(string),
		OIDCSecret:                d.Get("oidc_secret").(string),
		OverrideActiveTenant:      d.Get("override_active_tenant").(bool),
		SubAccountAccessLimit:     d.Get("sub_account_access_limit").(int),
		SkipEmailDomainValidation: d.Get("skip_email_domain_validation").(bool),
	}
}

func oidcConfigDeserialize(d *schema.ResourceData, f fronteggTenantSSOConfig) error {
	if err := setCommonSSOFields(d, f); err != nil {
		return err
	}
	for k, v := range map[string]interface{}{
		"oidc_client_id": f.OIDCClientID,
		"oidc_secret":    f.OIDCSecret,
	} {
		if err := d.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}

func resourceFronteggTenantOIDCConfigCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return tenantSSOCreate(ctx, d, meta, oidcConfigSerialize, oidcConfigDeserialize)
}

func resourceFronteggTenantOIDCConfigRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return tenantSSORead(ctx, d, meta, oidcConfigDeserialize)
}

func resourceFronteggTenantOIDCConfigUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return tenantSSOUpdate(ctx, d, meta, oidcConfigSerialize, resourceFronteggTenantOIDCConfigRead)
}
