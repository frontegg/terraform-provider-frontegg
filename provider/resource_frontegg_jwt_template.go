package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggJWTTemplatePath = "/identity/resources/jwt-templates/v1"

type fronteggJWTTemplateSchema struct {
	Claims map[string]interface{} `json:"claims"`
}

type fronteggJWTTemplate struct {
	ID             string                    `json:"id,omitempty"`
	VendorID       string                    `json:"vendorId,omitempty"`
	Key            string                    `json:"key,omitempty"`
	Name           string                    `json:"name,omitempty"`
	Description    string                    `json:"description,omitempty"`
	Expiration     int                       `json:"expiration"`
	Algorithm      string                    `json:"algorithm,omitempty"`
	TemplateSchema fronteggJWTTemplateSchema `json:"templateSchema"`
	CreatedAt      string                    `json:"createdAt,omitempty"`
	UpdatedAt      string                    `json:"updatedAt,omitempty"`
}

func resourceFronteggJWTTemplate() *schema.Resource {
	return &schema.Resource{
		Description:   `Configures a Frontegg JWT template.`,
		CreateContext: resourceFronteggJWTTemplateCreate,
		ReadContext:   resourceFronteggJWTTemplateRead,
		UpdateContext: resourceFronteggJWTTemplateUpdate,
		DeleteContext: resourceFronteggJWTTemplateDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"key": {
				Description: "A unique identifier key for the JWT template.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"name": {
				Description: "A human-readable name for the JWT template.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"description": {
				Description: "A human-readable description of the JWT template.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"expiration": {
				Description:  "The token expiration time in seconds.",
				Type:         schema.TypeInt,
				Required:     true,
				ValidateFunc: validation.IntAtLeast(1),
			},
			"algorithm": {
				Description: "The JWT signing algorithm. Valid values are `RS256` and `HS256`.",
				Type:        schema.TypeString,
				Required:    true,
				ValidateFunc: validation.StringInSlice([]string{
					"RS256",
					"HS256",
				}, false),
			},
			"claims": {
				Description: "Key-value pairs representing the JWT claims included in the template.",
				Type:        schema.TypeMap,
				Required:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"vendor_id": {
				Description: "The ID of the vendor that owns the JWT template.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"created_at": {
				Description: "The timestamp at which the JWT template was created.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"updated_at": {
				Description: "The timestamp at which the JWT template was last updated.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func resourceFronteggJWTTemplateSerialize(d *schema.ResourceData) fronteggJWTTemplate {
	return fronteggJWTTemplate{
		Key:         d.Get("key").(string),
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
		Expiration:  d.Get("expiration").(int),
		Algorithm:   d.Get("algorithm").(string),
		TemplateSchema: fronteggJWTTemplateSchema{
			Claims: d.Get("claims").(map[string]interface{}),
		},
	}
}

func resourceFronteggJWTTemplateDeserialize(d *schema.ResourceData, t fronteggJWTTemplate) error {
	d.SetId(t.ID)
	if err := d.Set("key", t.Key); err != nil {
		return err
	}
	if err := d.Set("name", t.Name); err != nil {
		return err
	}
	if err := d.Set("description", t.Description); err != nil {
		return err
	}
	if err := d.Set("expiration", t.Expiration); err != nil {
		return err
	}
	if err := d.Set("algorithm", t.Algorithm); err != nil {
		return err
	}
	if err := d.Set("vendor_id", t.VendorID); err != nil {
		return err
	}
	if err := d.Set("created_at", t.CreatedAt); err != nil {
		return err
	}
	if err := d.Set("updated_at", t.UpdatedAt); err != nil {
		return err
	}
	stringClaims := make(map[string]string, len(t.TemplateSchema.Claims))
	for k, v := range t.TemplateSchema.Claims {
		sv, ok := v.(string)
		if !ok {
			return fmt.Errorf("jwt template claim %q has unexpected non-string value of type %T; only string claim values are supported", k, v)
		}
		stringClaims[k] = sv
	}
	if err := d.Set("claims", stringClaims); err != nil {
		return err
	}
	return nil
}

func resourceFronteggJWTTemplateCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	in := resourceFronteggJWTTemplateSerialize(d)
	var out fronteggJWTTemplate
	if err := clientHolder.ApiClient.Post(ctx, fronteggJWTTemplatePath, in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggJWTTemplateDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggJWTTemplateRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	var out fronteggJWTTemplate
	err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s", fronteggJWTTemplatePath, d.Id()), &out)
	if err != nil {
		if restclient.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	if err := resourceFronteggJWTTemplateDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggJWTTemplateUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	in := resourceFronteggJWTTemplateSerialize(d)
	var out fronteggJWTTemplate
	if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s", fronteggJWTTemplatePath, d.Id()), in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggJWTTemplateDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggJWTTemplateDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggJWTTemplatePath, d.Id()), nil)
	if err != nil && !restclient.IsNotFound(err) {
		return diag.FromErr(err)
	}
	return nil
}
