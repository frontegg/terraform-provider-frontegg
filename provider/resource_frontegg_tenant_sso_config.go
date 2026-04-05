package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// fronteggTenantSSOConfigV2Path is used for creating SSO configurations (POST only).
const fronteggTenantSSOConfigV2Path = "/team/resources/sso/v2/configurations"

// fronteggTenantSSOConfigPath is the v1 path used for read, update, and delete operations.
const fronteggTenantSSOConfigPath = "/team/resources/sso/v1/configurations"

// fronteggTenantSSOConfig is the shared API request/response struct for both SAML and OIDC.
type fronteggTenantSSOConfig struct {
	ID                        string `json:"id,omitempty"`
	TenantID                  string `json:"tenantId,omitempty"`
	Enabled                   bool   `json:"enabled"`
	SSOEndpoint               string `json:"ssoEndpoint,omitempty"`
	PublicCertificate         string `json:"publicCertificate,omitempty"`
	SignRequest               bool   `json:"signRequest"`
	ACSUrl                    string `json:"acsUrl,omitempty"`
	SPEntityID                string `json:"spEntityId,omitempty"`
	Type                      string `json:"type,omitempty"`
	OIDCClientID              string `json:"oidcClientId,omitempty"`
	OIDCSecret                string `json:"oidcSecret,omitempty"`
	OverrideActiveTenant      bool   `json:"overrideActiveTenant"`
	SubAccountAccessLimit     int    `json:"subAccountAccessLimit,omitempty"`
	IDPClientID               string `json:"idpClientId,omitempty"`
	IDPClientSecret           string `json:"idpClientSecret,omitempty"`
	GeneratedVerification     string `json:"generatedVerification,omitempty"`
	SkipEmailDomainValidation bool   `json:"skipEmailDomainValidation"`
	CreatedAt                 string `json:"createdAt,omitempty"`
	UpdatedAt                 string `json:"updatedAt,omitempty"`
}

// tenantSSOHeaders builds the per-tenant header required by v1 SSO API operations.
func tenantSSOHeaders(tenantID string) (http.Header, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required but is empty; use 'tenant_id:resource_id' format when importing")
	}
	h := http.Header{}
	h.Set("frontegg-tenant-id", tenantID)
	return h, nil
}

// commonSSOSchema returns schema fields shared by both SAML and OIDC resources.
func commonSSOSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"tenant_id": {
			Description: "The ID of the tenant that owns this SSO configuration.",
			Type:        schema.TypeString,
			Required:    true,
			ForceNew:    true,
		},
		"enabled": {
			Description: "Whether the SSO configuration is enabled.",
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     true,
		},
		"sso_endpoint": {
			Description: "The IdP's login or authorization endpoint URL.",
			Type:        schema.TypeString,
			Optional:    true,
		},
		"override_active_tenant": {
			Description: "Whether to override the active tenant for users matched by this SSO configuration.",
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     false,
		},
		"sub_account_access_limit": {
			Description: "Limits which sub-accounts can access via this SSO configuration.",
			Type:        schema.TypeInt,
			Optional:    true,
		},
		"skip_email_domain_validation": {
			Description: "When true, users can authenticate via this SSO configuration even if the associated email domain has not been validated through DNS TXT record verification.",
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     false,
		},
		"generated_verification": {
			Description: "A computed token used to verify domain ownership. Add this value as a DNS TXT record on your domain to validate it.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		"created_at": {
			Description: "When the SSO configuration was created.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		"updated_at": {
			Description: "When the SSO configuration was last updated.",
			Type:        schema.TypeString,
			Computed:    true,
		},
	}
}

// tenantSSOCreate is the shared Create handler for both SAML and OIDC resources.
// Uses v2 API which requires tenantId in the request body (no header).
func tenantSSOCreate(ctx context.Context, d *schema.ResourceData, meta interface{}, serialize func(*schema.ResourceData) fronteggTenantSSOConfig, deserialize func(*schema.ResourceData, fronteggTenantSSOConfig) error) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	in := serialize(d)
	in.TenantID = d.Get("tenant_id").(string)
	var out fronteggTenantSSOConfig
	if err := clientHolder.ApiClient.Post(ctx, fronteggTenantSSOConfigV2Path, in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := deserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

// tenantSSORead is the shared Read handler for both SAML and OIDC resources.
// Uses v1 API with frontegg-tenant-id header.
func tenantSSORead(ctx context.Context, d *schema.ResourceData, meta interface{}, deserialize func(*schema.ResourceData, fronteggTenantSSOConfig) error) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	headers, err := tenantSSOHeaders(d.Get("tenant_id").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	var configs []fronteggTenantSSOConfig
	if err := clientHolder.ApiClient.GetWithHeaders(ctx, fronteggTenantSSOConfigPath, headers, &configs); err != nil {
		return diag.FromErr(err)
	}
	for _, c := range configs {
		if c.ID == d.Id() {
			if err := deserialize(d, c); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}
	d.SetId("")
	return nil
}

// tenantSSOUpdate is the shared Update handler for both SAML and OIDC resources.
// Uses v2 API which requires tenantId in the request body (no header).
func tenantSSOUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}, serialize func(*schema.ResourceData) fronteggTenantSSOConfig, read func(context.Context, *schema.ResourceData, interface{}) diag.Diagnostics) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	in := serialize(d)
	in.TenantID = d.Get("tenant_id").(string)
	if err := clientHolder.ApiClient.Patch(ctx, fmt.Sprintf("%s/%s", fronteggTenantSSOConfigV2Path, d.Id()), in, nil); err != nil {
		return diag.FromErr(err)
	}
	return read(ctx, d, meta)
}

// tenantSSODelete is the shared Delete handler for both SAML and OIDC resources.
// Uses v1 API with frontegg-tenant-id header.
func tenantSSODelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	headers, err := tenantSSOHeaders(d.Get("tenant_id").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	if err := clientHolder.ApiClient.DeleteWithHeaders(ctx, fmt.Sprintf("%s/%s", fronteggTenantSSOConfigPath, d.Id()), headers, nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

// tenantSSOImport is the shared Import handler for both SAML and OIDC resources.
func tenantSSOImport(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	parts := strings.SplitN(d.Id(), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid import ID format: expected 'tenant_id:config_id', got %q", d.Id())
	}
	if err := d.Set("tenant_id", parts[0]); err != nil {
		return nil, err
	}
	d.SetId(parts[1])
	return []*schema.ResourceData{d}, nil
}

// setCommonSSOFields writes API response fields that are common to both SAML and OIDC.
func setCommonSSOFields(d *schema.ResourceData, f fronteggTenantSSOConfig) error {
	d.SetId(f.ID)
	for k, v := range map[string]interface{}{
		"enabled":                      f.Enabled,
		"sso_endpoint":                 f.SSOEndpoint,
		"override_active_tenant":       f.OverrideActiveTenant,
		"sub_account_access_limit":     f.SubAccountAccessLimit,
		"skip_email_domain_validation": f.SkipEmailDomainValidation,
		"generated_verification":       f.GeneratedVerification,
		"created_at":                   f.CreatedAt,
		"updated_at":                   f.UpdatedAt,
	} {
		if err := d.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}
