package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const fronteggTenantSSODomainForceValidatePath = "/team/resources/sso/v1/configurations/domains"

func resourceFronteggTenantSSODomainValidation() *schema.Resource {
	return &schema.Resource{
		Description: `Force-validates an SSO domain for a tenant without requiring DNS TXT record verification. Use this when provisioning SSO programmatically for trusted tenants where DNS verification is impractical.`,

		CreateContext: resourceFronteggTenantSSODomainValidationCreate,
		ReadContext:   resourceFronteggTenantSSODomainValidationRead,
		UpdateContext: resourceFronteggTenantSSODomainValidationUpdate,
		DeleteContext: resourceFronteggTenantSSODomainValidationDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceFronteggTenantSSODomainValidationImport,
		},

		Schema: map[string]*schema.Schema{
			"domain": {
				Description: "The domain name to force-validate (e.g. `example.com`).",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"validated": {
				Description: "Whether to mark the domain as validated.",
				Type:        schema.TypeBool,
				Required:    true,
			},
			"tenant_id": {
				Description: "The ID of the tenant that owns the SSO domain.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
		},
	}
}

func resourceFronteggTenantSSODomainValidationImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	parts := strings.SplitN(d.Id(), ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid import format, expected tenant_id:domain, got: %s", d.Id())
	}
	tenantID := parts[0]
	domain := parts[1]

	if err := d.Set("tenant_id", tenantID); err != nil {
		return nil, err
	}
	if err := d.Set("domain", domain); err != nil {
		return nil, err
	}
	d.SetId(fmt.Sprintf("%s:%s", tenantID, domain))

	return []*schema.ResourceData{d}, nil
}

type fronteggForceValidateDomainRequest struct {
	Validated bool   `json:"validated"`
	TenantID  string `json:"tenantId"`
}

func resourceFronteggTenantSSODomainValidationApply(ctx context.Context, d *schema.ResourceData, meta interface{}, validated bool) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	domain := d.Get("domain").(string)
	tenantID := d.Get("tenant_id").(string)

	in := fronteggForceValidateDomainRequest{
		Validated: validated,
		TenantID:  tenantID,
	}

	if err := clientHolder.ApiClient.Put(
		ctx,
		fmt.Sprintf("%s/%s/force-validate", fronteggTenantSSODomainForceValidatePath, domain),
		in,
		nil,
	); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%s:%s", tenantID, domain))
	if err := d.Set("validated", validated); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggTenantSSODomainValidationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceFronteggTenantSSODomainValidationApply(ctx, d, meta, d.Get("validated").(bool))
}

func resourceFronteggTenantSSODomainValidationRead(_ context.Context, _ *schema.ResourceData, _ interface{}) diag.Diagnostics {
	// Validation state is fully managed; there is no read endpoint.
	return nil
}

func resourceFronteggTenantSSODomainValidationUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceFronteggTenantSSODomainValidationApply(ctx, d, meta, d.Get("validated").(bool))
}

func resourceFronteggTenantSSODomainValidationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceFronteggTenantSSODomainValidationApply(ctx, d, meta, false)
}
