package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const fronteggTenantPath = "/tenants/resources/tenants/v1"

type fronteggTenant struct {
	Key            string `json:"tenantId,omitempty"`
	Name           string `json:"name,omitempty"`
	ApplicationUri string `json:"applicationUrl,omitempty"`
}

func resourceFronteggTenant() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg tenant.`,

		CreateContext: resourceFronteggTenantCreate,
		ReadContext:   resourceFronteggTenantRead,
		UpdateContext: resourceFronteggTenantUpdate,
		DeleteContext: resourceFronteggTenantDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "A human-readable name for the tenant.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"key": {
				Description: "A human-readable identifier for the tenant.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"application_uri": {
				Description: "The application URI for this tenant.",
				Type:        schema.TypeString,
				Required:    true,
			},
		},
	}
}

func resourceFronteggTenantSerialize(d *schema.ResourceData) fronteggTenant {
	return fronteggTenant{
		Name:           d.Get("name").(string),
		Key:            d.Get("key").(string),
		ApplicationUri: d.Get("application_uri").(string),
	}
}

func resourceFronteggTenantDeserialize(d *schema.ResourceData, f fronteggTenant) error {
	d.SetId(f.Key)
	if err := d.Set("name", f.Name); err != nil {
		return err
	}
	if err := d.Set("key", f.Key); err != nil {
		return err
	}
	if err := d.Set("application_uri", f.ApplicationUri); err != nil {
		return err
	}
	return nil
}

func resourceFronteggTenantCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggTenantSerialize(d)
	var out fronteggTenant
	if err := clientHolder.ApiClient.Post(ctx, fronteggTenantPath, in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggTenantDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggTenantRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	var out []fronteggTenant
	if err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s", fronteggTenantPath, d.Id()), &out); err != nil {
		return diag.FromErr(err)
	}
	for _, c := range out {
		if c.Key == d.Id() {
			if err := resourceFronteggTenantDeserialize(d, c); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}
	d.SetId("")
	return nil
}

func resourceFronteggTenantUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	var out fronteggTenant
	in := resourceFronteggTenantSerialize(d)
	if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s", fronteggTenantPath, d.Id()), in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggTenantDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggTenantDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggTenantPath, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
