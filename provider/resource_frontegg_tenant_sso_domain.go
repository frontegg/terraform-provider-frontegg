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

type fronteggTenantSSODomain struct {
	ID        string `json:"id,omitempty"`
	Domain    string `json:"domain"`
	Validated bool   `json:"validated"`
	TxtRecord string `json:"txtRecord,omitempty"`
}

type fronteggTenantSSOConfigWithDomains struct {
	ID      string                    `json:"id,omitempty"`
	Domains []fronteggTenantSSODomain `json:"domains"`
}

func resourceFronteggTenantSSODomain() *schema.Resource {
	return &schema.Resource{
		Description: `Associates an email domain with a tenant SSO configuration. Users with email addresses matching this domain will be redirected to the SSO IdP for authentication. After creating the domain, validate ownership by adding the ` + "`txt_record`" + ` value as a DNS TXT record.`,

		CreateContext: resourceFronteggTenantSSODomainCreate,
		ReadContext:   resourceFronteggTenantSSODomainRead,
		DeleteContext: resourceFronteggTenantSSODomainDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceFronteggTenantSSODomainImport,
		},

		Schema: map[string]*schema.Schema{
			"tenant_id": {
				Description: "The ID of the tenant that owns the SSO configuration.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"sso_config_id": {
				Description: "The ID of the SSO configuration to attach the domain to. Can be the ID of a `frontegg_tenant_saml_config` or `frontegg_tenant_oidc_config` resource.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"domain": {
				Description: "The domain name to add to the SSO configuration.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"validated": {
				Description: "Whether the domain ownership has been confirmed via DNS TXT record verification.",
				Type:        schema.TypeBool,
				Computed:    true,
			},
			"txt_record": {
				Description: "The DNS TXT record value used to validate the domain.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func resourceFronteggTenantSSODomainImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	parts := strings.SplitN(d.Id(), ":", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid import format, expected tenant_id:sso_config_id:domain_id, got: %s", d.Id())
	}

	tenantID := parts[0]
	ssoConfigID := parts[1]
	domainID := parts[2]

	if err := d.Set("tenant_id", tenantID); err != nil {
		return nil, err
	}
	if err := d.Set("sso_config_id", ssoConfigID); err != nil {
		return nil, err
	}
	d.SetId(domainID)

	return []*schema.ResourceData{d}, nil
}

func resourceFronteggTenantSSODomainCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	tenantID := d.Get("tenant_id").(string)
	if tenantID == "" {
		return diag.Errorf("tenant_id is required but is empty; use 'tenant_id:sso_config_id:domain_id' format when importing")
	}
	ssoConfigID := d.Get("sso_config_id").(string)
	domain := d.Get("domain").(string)

	headers := http.Header{}
	headers.Add("frontegg-tenant-id", tenantID)

	in := struct {
		Domain string `json:"domain"`
	}{
		Domain: domain,
	}

	var out fronteggTenantSSODomain
	if err := clientHolder.ApiClient.PostWithHeaders(
		ctx,
		fmt.Sprintf("%s/%s/domains", fronteggTenantSSOConfigPath, ssoConfigID),
		headers,
		in,
		&out,
	); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(out.ID)
	if err := d.Set("validated", out.Validated); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("txt_record", out.TxtRecord); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggTenantSSODomainRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	tenantID := d.Get("tenant_id").(string)
	if tenantID == "" {
		return diag.Errorf("tenant_id is required but is empty; use 'tenant_id:sso_config_id:domain_id' format when importing")
	}
	ssoConfigID := d.Get("sso_config_id").(string)

	headers := http.Header{}
	headers.Add("frontegg-tenant-id", tenantID)

	var configs []fronteggTenantSSOConfigWithDomains
	if err := clientHolder.ApiClient.GetWithHeaders(
		ctx,
		fronteggTenantSSOConfigPath,
		headers,
		&configs,
	); err != nil {
		return diag.FromErr(err)
	}

	// Find the config matching sso_config_id.
	var targetConfig *fronteggTenantSSOConfigWithDomains
	for i := range configs {
		if configs[i].ID == ssoConfigID {
			targetConfig = &configs[i]
			break
		}
	}
	if targetConfig == nil {
		d.SetId("")
		return nil
	}

	var found *fronteggTenantSSODomain
	for i := range targetConfig.Domains {
		if targetConfig.Domains[i].ID == d.Id() {
			found = &targetConfig.Domains[i]
			break
		}
	}

	if found == nil {
		d.SetId("")
		return nil
	}

	d.SetId(found.ID)
	if err := d.Set("domain", found.Domain); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("validated", found.Validated); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("txt_record", found.TxtRecord); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggTenantSSODomainDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	tenantID := d.Get("tenant_id").(string)
	if tenantID == "" {
		return diag.Errorf("tenant_id is required but is empty; use 'tenant_id:sso_config_id:domain_id' format when importing")
	}
	ssoConfigID := d.Get("sso_config_id").(string)

	headers := http.Header{}
	headers.Add("frontegg-tenant-id", tenantID)

	if err := clientHolder.ApiClient.DeleteWithHeaders(
		ctx,
		fmt.Sprintf("%s/%s/domains/%s", fronteggTenantSSOConfigPath, ssoConfigID, d.Id()),
		headers,
		nil,
	); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
