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
	Status		   string `json:"status,omitempty"`
	Website		   string `json:"website,omitempty"`
	Logo		   string `json:"logo,omitempty"`
	LogoUrl		   string `json:"logoUrl,omitempty"`
	Address		   string `json:"address,omitempty"`
	Timezone	   string `json:"timezone,omitempty"`
	Currency	   string `json:"currency,omitempty"`
	CreatorName	   string `json:"creatorName,omitempty"`
	CreatorEmail   string `json:"creatorEmail,omitempty"`
	IsReseller     bool `json:"isReseller,omitempty"`
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
				Optional:    true,
			},
			"status": {
				Description: "The status for this tenant.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"website": {
				Description: "The website for this tenant.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"logo": {
				Description: "The logo for this tenant.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"logo_url": {
				Description: "The logo Url for this tenant.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"address": {
				Description: "The address for this tenant.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"timezone": {
				Description: "The timezone for this tenant.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"currency": {
				Description: "The currency for this tenant.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"creator_name": {
				Description: "The creator name for this tenant.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"creator_email": {
				Description: "The creator email for this tenant.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"is_reseller": {
				Description: "Mark tenant as reseller",
				Type:        schema.TypeBool,
				Optional:    true,
			},
		},
	}
}

func resourceFronteggTenantSerialize(d *schema.ResourceData) fronteggTenant {
	return fronteggTenant{
		Name:           d.Get("name").(string),
		Key:            d.Get("key").(string),
		ApplicationUri: d.Get("application_uri").(string),
		Status: 		d.Get("status").(string),
		Website: 		d.Get("website").(string),
		Logo: 			d.Get("logo").(string),
		LogoUrl: 		d.Get("logo_url").(string),
		Address: 		d.Get("address").(string),
		Timezone: 		d.Get("timezone").(string),
		Currency: 		d.Get("currency").(string),
		CreatorName: 	d.Get("creator_name").(string),
		CreatorEmail: 	d.Get("creator_email").(string),
		IsReseller: 	d.Get("is_reseller").(bool),
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
	if err := d.Set("status", f.Status); err != nil {
		return err
	}
	if err := d.Set("website", f.Website); err != nil {
		return err
	}
	if err := d.Set("logo", f.Logo); err != nil {
		return err
	}
	if err := d.Set("logo_url", f.LogoUrl); err != nil {
		return err
	}
	if err := d.Set("address", f.Address); err != nil {
		return err
	}
	if err := d.Set("timezone", f.Timezone); err != nil {
		return err
	}
	if err := d.Set("currency", f.Currency); err != nil {
		return err
	}
	if err := d.Set("creator_name", f.CreatorName); err != nil {
		return err
	}
	if err := d.Set("creator_email", f.CreatorEmail); err != nil {
		return err
	}
	if err := d.Set("is_reseller", f.IsReseller); err != nil {
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
