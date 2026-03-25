package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceFronteggTenantSAMLConfig() *schema.Resource {
	s := commonSSOSchema()

	s["public_certificate"] = &schema.Schema{
		Description: "The IdP's X.509 public certificate (Base64-encoded). Used by Frontegg to verify the signature on incoming SAML assertions.",
		Type:        schema.TypeString,
		Optional:    true,
		Sensitive:   true,
	}
	s["sign_request"] = &schema.Schema{
		Description: "Whether Frontegg should cryptographically sign outgoing SAML authentication requests sent to the IdP.",
		Type:        schema.TypeBool,
		Optional:    true,
		Default:     false,
	}
	s["acs_url"] = &schema.Schema{
		Description: "The Assertion Consumer Service URL — the Frontegg endpoint that receives and processes SAML responses from the IdP. Register this value in your IdP's SAML application settings.",
		Type:        schema.TypeString,
		Computed:    true,
	}
	s["sp_entity_id"] = &schema.Schema{
		Description: "The Service Provider Entity ID — a unique URI that identifies Frontegg in the SAML exchange. Must match the audience restriction in the IdP's SAML assertion.",
		Type:        schema.TypeString,
		Optional:    true,
	}
	s["idp_client_id"] = &schema.Schema{
		Description: "The SSO application client ID used to authenticate group-fetch requests from the IdP (for SAML group-to-role mappings).",
		Type:        schema.TypeString,
		Optional:    true,
	}
	s["idp_client_secret"] = &schema.Schema{
		Description: "The client secret paired with `idp_client_id` for authenticating group-fetch requests.",
		Type:        schema.TypeString,
		Optional:    true,
		Sensitive:   true,
	}

	return &schema.Resource{
		Description: `Configures a SAML SSO configuration for a Frontegg tenant. Users whose email domain matches a domain associated with this configuration will be redirected to the SAML Identity Provider (IdP) for authentication.`,

		CreateContext: resourceFronteggTenantSAMLConfigCreate,
		ReadContext:   resourceFronteggTenantSAMLConfigRead,
		UpdateContext: resourceFronteggTenantSAMLConfigUpdate,
		DeleteContext: tenantSSODelete,
		Importer: &schema.ResourceImporter{
			StateContext: tenantSSOImport,
		},
		Schema: s,
	}
}

// samlCertToAPI base64-encodes the certificate if it isn't already, matching
// the Frontegg API's expectation (it stores and returns the cert as base64).
func samlCertToAPI(cert string) string {
	if cert == "" {
		return ""
	}
	if _, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cert)); err == nil {
		return cert // already base64
	}
	return base64.StdEncoding.EncodeToString([]byte(cert))
}

// samlCertFromAPI base64-decodes the certificate returned by the API back to
// PEM format so that state stores what the user originally provided.
func samlCertFromAPI(cert string) (string, error) {
	if cert == "" {
		return "", nil
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cert))
	if err != nil {
		return "", fmt.Errorf("API returned a public_certificate that is not valid base64: %w", err)
	}
	return string(b), nil
}

func samlConfigSerialize(d *schema.ResourceData) fronteggTenantSSOConfig {
	return fronteggTenantSSOConfig{
		Type:                      "saml",
		Enabled:                   d.Get("enabled").(bool),
		SSOEndpoint:               d.Get("sso_endpoint").(string),
		PublicCertificate:         samlCertToAPI(d.Get("public_certificate").(string)),
		SignRequest:               d.Get("sign_request").(bool),
		SPEntityID:                d.Get("sp_entity_id").(string),
		IDPClientID:               d.Get("idp_client_id").(string),
		IDPClientSecret:           d.Get("idp_client_secret").(string),
		OverrideActiveTenant:      d.Get("override_active_tenant").(bool),
		SubAccountAccessLimit:     d.Get("sub_account_access_limit").(int),
		SkipEmailDomainValidation: d.Get("skip_email_domain_validation").(bool),
	}
}

func samlConfigDeserialize(d *schema.ResourceData, f fronteggTenantSSOConfig) error {
	if err := setCommonSSOFields(d, f); err != nil {
		return err
	}
	decodedCert, err := samlCertFromAPI(f.PublicCertificate)
	if err != nil {
		return err
	}
	for k, v := range map[string]interface{}{
		"public_certificate": decodedCert,
		"sign_request":       f.SignRequest,
		"acs_url":            f.ACSUrl,
		"sp_entity_id":       f.SPEntityID,
		"idp_client_id":      f.IDPClientID,
		"idp_client_secret":  f.IDPClientSecret,
	} {
		if err := d.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}

func resourceFronteggTenantSAMLConfigCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return tenantSSOCreate(ctx, d, meta, samlConfigSerialize, samlConfigDeserialize)
}

func resourceFronteggTenantSAMLConfigRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return tenantSSORead(ctx, d, meta, samlConfigDeserialize)
}

func resourceFronteggTenantSAMLConfigUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return tenantSSOUpdate(ctx, d, meta, samlConfigSerialize, resourceFronteggTenantSAMLConfigRead)
}
