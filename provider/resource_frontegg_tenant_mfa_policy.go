package provider

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggMFAPolicyURL = "/identity/resources/configurations/v1/mfa-policy"

type fronteggMFAPolicy struct {
	ID                    string `json:"id,omitempty"`
	EnforceMFAType        string `json:"enforceMFAType"`
	AllowRememberMyDevice bool   `json:"allowRememberMyDevice"`
	MFADeviceExpiration   int    `json:"mfaDeviceExpiration"`
	CreatedAt             string `json:"createdAt,omitempty"`
	UpdatedAt             string `json:"updatedAt,omitempty"`
}

func resourceFronteggTenantMFAPolicy() *schema.Resource {
	return &schema.Resource{
		Description: `Configures the MFA policy for a Frontegg tenant.

This is a singleton resource per tenant. You must only create one frontegg_tenant_mfa_policy resource
per tenant.

**Note:** This resource cannot be deleted. When destroyed, Terraform will remove it from the state file, but the MFA policy will remain in its last-applied state.`,

		CreateContext: resourceFronteggTenantMFAPolicyCreate,
		ReadContext:   resourceFronteggTenantMFAPolicyRead,
		UpdateContext: resourceFronteggTenantMFAPolicyUpdate,
		DeleteContext: resourceFronteggTenantMFAPolicyDelete,
		Importer: &schema.ResourceImporter{
			StateContext: tenantMFAPolicyImport,
		},

		Schema: map[string]*schema.Schema{
			"tenant_id": {
				Description: "The ID of the tenant for which to configure the MFA policy.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"enforce_mfa_type": {
				Description: `Whether to force use of MFA.

Must be one of "off", "on", or "unless-saml".`,
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{"off", "on", "unless-saml"}, false),
			},
			"allow_remember_my_device": {
				Description: "Whether to allow users to remember their device to skip MFA on subsequent logins.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
			"mfa_device_expiration": {
				Description: "The number of seconds that MFA devices can be remembered for, if allow_remember_my_device is true.",
				Type:        schema.TypeInt,
				Required:    true,
			},
		},
	}
}

func serializeMFAEnforce(s string) string {
	switch s {
	case "off":
		return "DontForce"
	case "on":
		return "Force"
	case "unless-saml":
		return "ForceExceptSAML"
	}
	panic("unreachable")
}

func deserializeMFAEnforce(s string) string {
	switch s {
	case "DontForce":
		return "off"
	case "Force":
		return "on"
	case "ForceExceptSAML":
		return "unless-saml"
	default:
		return "off"
	}
}

// getMFAPolicy fetches the MFA policy. Pass a non-nil header to scope to a tenant.
func getMFAPolicy(ctx context.Context, client *restclient.Client, headers http.Header) (fronteggMFAPolicy, error) {
	var out fronteggMFAPolicy
	if err := client.GetWithHeaders(ctx, fronteggMFAPolicyURL, headers, &out); err != nil {
		return out, err
	}
	return out, nil
}

// writeMFAPolicy upserts the MFA policy. Pass a non-nil header to scope to a tenant.
func writeMFAPolicy(ctx context.Context, client *restclient.Client, headers http.Header, in fronteggMFAPolicy) error {
	client.ConflictRetryMethod("PATCH")
	return client.PostWithHeaders(ctx, fronteggMFAPolicyURL, headers, in, nil)
}

func resourceFronteggTenantMFAPolicyCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	tenantID := d.Get("tenant_id").(string)
	if tenantID == "" {
		return diag.Errorf("tenant_id is required but is empty")
	}
	headers := http.Header{}
	headers.Add("frontegg-tenant-id", tenantID)
	in := fronteggMFAPolicy{
		EnforceMFAType:        serializeMFAEnforce(d.Get("enforce_mfa_type").(string)),
		AllowRememberMyDevice: d.Get("allow_remember_my_device").(bool),
		MFADeviceExpiration:   d.Get("mfa_device_expiration").(int),
	}
	if err := writeMFAPolicy(ctx, &clientHolder.ApiClient, headers, in); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(tenantID)
	return resourceFronteggTenantMFAPolicyRead(ctx, d, meta)
}

func resourceFronteggTenantMFAPolicyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	tenantID := d.Get("tenant_id").(string)
	if tenantID == "" {
		return diag.Errorf("tenant_id is required but is empty")
	}
	headers := http.Header{}
	headers.Add("frontegg-tenant-id", tenantID)
	out, err := getMFAPolicy(ctx, &clientHolder.ApiClient, headers)
	if err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("enforce_mfa_type", deserializeMFAEnforce(out.EnforceMFAType)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("allow_remember_my_device", out.AllowRememberMyDevice); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("mfa_device_expiration", out.MFADeviceExpiration); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggTenantMFAPolicyUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	tenantID := d.Get("tenant_id").(string)
	if tenantID == "" {
		return diag.Errorf("tenant_id is required but is empty")
	}
	headers := http.Header{}
	headers.Add("frontegg-tenant-id", tenantID)
	in := fronteggMFAPolicy{
		EnforceMFAType:        serializeMFAEnforce(d.Get("enforce_mfa_type").(string)),
		AllowRememberMyDevice: d.Get("allow_remember_my_device").(bool),
		MFADeviceExpiration:   d.Get("mfa_device_expiration").(int),
	}
	if err := writeMFAPolicy(ctx, &clientHolder.ApiClient, headers, in); err != nil {
		return diag.FromErr(err)
	}
	return resourceFronteggTenantMFAPolicyRead(ctx, d, meta)
}

func resourceFronteggTenantMFAPolicyDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Printf("[WARN] Cannot destroy tenant MFA policy. Terraform will remove this resource from the " +
		"state file, but the MFA policy will remain in its last-applied state.")
	return nil
}

func tenantMFAPolicyImport(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	tenantID := d.Id()
	if tenantID == "" {
		return nil, fmt.Errorf("invalid import ID: tenant_id cannot be empty")
	}
	if err := d.Set("tenant_id", tenantID); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}
