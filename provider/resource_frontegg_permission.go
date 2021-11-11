package provider

import (
	"context"
	"fmt"

	"github.com/benesch/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const fronteggPermissionPath = "/identity/resources/permissions/v1"

type fronteggPermission struct {
	ID          string `json:"id,omitempty"`
	CategoryID  string `json:"categoryId,omitempty"`
	Name        string `json:"name,omitempty"`
	Key         string `json:"key,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
}

func resourceFronteggPermission() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg permission.`,

		CreateContext: resourceFronteggPermissionCreate,
		ReadContext:   resourceFronteggPermissionRead,
		UpdateContext: resourceFronteggPermissionUpdate,
		DeleteContext: resourceFronteggPermissionDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "A human-readable name for the permission.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"key": {
				Description: "A human-readable identifier for the permission.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"category_id": {
				Description: "The identifier of the category to which this permission belongs.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"description": {
				Description: "A human-readable description of the permission.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"created_at": {
				Description: "The timestamp at which the permission was created.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func resourceFronteggPermissionSerialize(d *schema.ResourceData) fronteggPermission {
	return fronteggPermission{
		Name:        d.Get("name").(string),
		Key:         d.Get("key").(string),
		CategoryID:  d.Get("category_id").(string),
		Description: d.Get("description").(string),
	}
}

func resourceFronteggPermissionDeserialize(d *schema.ResourceData, f fronteggPermission) error {
	d.SetId(f.ID)
	if err := d.Set("name", f.Name); err != nil {
		return err
	}
	if err := d.Set("key", f.Key); err != nil {
		return err
	}
	if err := d.Set("category_id", f.CategoryID); err != nil {
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

func resourceFronteggPermissionCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := []fronteggPermission{resourceFronteggPermissionSerialize(d)}
	var out []fronteggPermission
	if err := clientHolder.ApiClient.Post(ctx, fronteggPermissionPath, in, &out); err != nil {
		return diag.FromErr(err)
	}
	if len(out) != 1 {
		return diag.Errorf("server returned unexpected number of results when creating permission: %d", len(out))
	}
	if err := resourceFronteggPermissionDeserialize(d, out[0]); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggPermissionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	var out []fronteggPermission
	if err := clientHolder.ApiClient.Get(ctx, fronteggPermissionPath, &out); err != nil {
		return diag.FromErr(err)
	}
	for _, c := range out {
		if c.ID == d.Id() {
			if err := resourceFronteggPermissionDeserialize(d, c); err != nil {
				return diag.FromErr(err)
			}
			return diag.Diagnostics{}
		}
	}
	d.SetId("")
	return nil
}

func resourceFronteggPermissionUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggPermissionSerialize(d)
	if err := clientHolder.ApiClient.Patch(ctx, fmt.Sprintf("%s/%s", fronteggPermissionPath, d.Id()), in, nil); err != nil {
		return diag.FromErr(err)
	}
	return resourceFronteggPermissionRead(ctx, d, meta)
}

func resourceFronteggPermissionDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggPermissionPath, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
