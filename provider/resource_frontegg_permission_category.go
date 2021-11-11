package provider

import (
	"context"
	"fmt"

	"github.com/benesch/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const fronteggPermissionCategoryPath = "/identity/resources/permissions/v1/categories"

type fronteggPermissionCategory struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
}

func resourceFronteggPermissionCategory() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg permission category.`,

		CreateContext: resourceFronteggPermissionCategoryCreate,
		ReadContext:   resourceFronteggPermissionCategoryRead,
		UpdateContext: resourceFronteggPermissionCategoryUpdate,
		DeleteContext: resourceFronteggPermissionCategoryDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "A human-readable name for the permission category.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"description": {
				Description: "A human-readable description of the permission category.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"created_at": {
				Description: "The timestamp at which the permission category was created.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func resourceFronteggPermissionCategorySerialize(d *schema.ResourceData) fronteggPermissionCategory {
	return fronteggPermissionCategory{
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
	}
}

func resourceFronteggPermissionCategoryDeserialize(d *schema.ResourceData, f fronteggPermissionCategory) error {
	d.SetId(f.ID)
	if err := d.Set("name", f.Name); err != nil {
		return err
	}
	if err := d.Set("description", f.Description); err != nil {
		return err
	}
	if err := d.Set("created_at", f.CreatedAt); err != nil {
		return err
	}
	return nil
}

func resourceFronteggPermissionCategoryCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggPermissionCategorySerialize(d)
	var out fronteggPermissionCategory
	if err := clientHolder.ApiClient.Post(ctx, fronteggPermissionCategoryPath, in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggPermissionCategoryDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggPermissionCategoryRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	var out []fronteggPermissionCategory
	if err := clientHolder.ApiClient.Get(ctx, fronteggPermissionCategoryPath, &out); err != nil {
		return diag.FromErr(err)
	}
	for _, c := range out {
		if c.ID == d.Id() {
			if err := resourceFronteggPermissionCategoryDeserialize(d, c); err != nil {
				return diag.FromErr(err)
			}
			return diag.Diagnostics{}
		}
	}
	d.SetId("")
	return nil
}

func resourceFronteggPermissionCategoryUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggPermissionCategorySerialize(d)
	if err := clientHolder.ApiClient.Patch(ctx, fmt.Sprintf("%s/%s", fronteggPermissionCategoryPath, d.Id()), in, nil); err != nil {
		return diag.FromErr(err)
	}
	return resourceFronteggPermissionCategoryRead(ctx, d, meta)
}

func resourceFronteggPermissionCategoryDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggPermissionCategoryPath, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
