package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const fronteggRolePath = "/identity/resources/roles/v1"

type fronteggRole struct {
	ID            string   `json:"id,omitempty"`
	Name          string   `json:"name,omitempty"`
	Key           string   `json:"key,omitempty"`
	Description   string   `json:"description,omitempty"`
	Level         int      `json:"level"`
	IsDefault     bool     `json:"isDefault"`
	FirstUserRole bool     `json:"firstUserRole"`
	Permissions   []string `json:"permissions"`
	TenantID      string   `json:"tenantId,omitempty"`
	VendorID      string   `json:"vendorId,omitempty"`
	CreatedAt     string   `json:"createdAt,omitempty"`
}

type fronteggRolePermissions struct {
	PermissionIDs []string `json:"permissionIds"`
}

func resourceFronteggRole() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg role.`,

		CreateContext: resourceFronteggRoleCreate,
		ReadContext:   resourceFronteggRoleRead,
		UpdateContext: resourceFronteggRoleUpdate,
		DeleteContext: resourceFronteggRoleDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "A human-readable name for the role.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"key": {
				Description: "A human-readable identifier for the role.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"description": {
				Description: "A human-readable description of the role.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"default": {
				Description: "Whether the role should be applied to new users by default.",
				Type:        schema.TypeBool,
				Required:    true,
			},
			"first_user": {
				Description: "Whether the role should be applied to the first user in the tenant (new tenants only).",
				Type:        schema.TypeBool,
				Optional:    true,
			},
			"level": {
				Description: "The level of the role in the role hierarchy.",
				Type:        schema.TypeInt,
				Required:    true,
			},
			"permission_ids": {
				Description: "The IDs of the permissions that the role confers to its members.",
				Type:        schema.TypeSet,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Required:    true,
			},
			"tenant_id": {
				Description: "The ID of the tenant that owns the role.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"vendor_id": {
				Description: "The ID of the vendor that owns the role.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"created_at": {
				Description: "The timestamp at which the role was created.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func resourceFronteggRoleSerialize(d *schema.ResourceData) fronteggRole {
	return fronteggRole{
		Name:          d.Get("name").(string),
		IsDefault:     d.Get("default").(bool),
		FirstUserRole: d.Get("first_user").(bool),
		Key:           d.Get("key").(string),
		Description:   d.Get("description").(string),
		Level:         d.Get("level").(int),
	}
}

func resourceFronteggRolePermissionsSerialize(d *schema.ResourceData) fronteggRolePermissions {
	return fronteggRolePermissions{
		PermissionIDs: stringSetToList(d.Get("permission_ids").(*schema.Set)),
	}
}

func resourceFronteggRoleDeserialize(d *schema.ResourceData, f fronteggRole) error {
	d.SetId(f.ID)
	if err := d.Set("name", f.Name); err != nil {
		return err
	}
	if err := d.Set("key", f.Key); err != nil {
		return err
	}
	if err := d.Set("description", f.Description); err != nil {
		return err
	}
	if err := d.Set("default", f.IsDefault); err != nil {
		return err
	}
	if err := d.Set("first_user", f.FirstUserRole); err != nil {
		return err
	}
	if err := d.Set("level", f.Level); err != nil {
		return err
	}
	if err := d.Set("permission_ids", f.Permissions); err != nil {
		return err
	}
	if err := d.Set("tenant_id", f.TenantID); err != nil {
		return err
	}
	if err := d.Set("vendor_id", f.VendorID); err != nil {
		return err
	}
	if err := d.Set("created_at", f.CreatedAt); err != nil {
		return err
	}
	return nil
}

func getTenantIdHeaders(d *schema.ResourceData) http.Header {
	headers := http.Header{}
	tenant_id := d.Get("tenant_id").(string)
	if tenant_id != "" {
		headers.Add("frontegg-tenant-id", tenant_id)
	} else {
		headers = nil
	}
	return headers
}

func resourceFronteggRoleCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	headers := getTenantIdHeaders(d)
	clientHolder := meta.(*restclient.ClientHolder)
	var id string
	{
		in := []fronteggRole{resourceFronteggRoleSerialize(d)}
		var out []fronteggRole
		if err := clientHolder.ApiClient.PostWithHeaders(ctx, fronteggRolePath, headers, in, &out); err != nil {
			return diag.FromErr(err)
		}
		if len(out) != 1 {
			return diag.Errorf("server returned unexpected number of results when creating Role: %d", len(out))
		}
		id = out[0].ID
	}
	var out fronteggRole
	in := resourceFronteggRolePermissionsSerialize(d)
	if err := clientHolder.ApiClient.PutWithHeaders(ctx, fmt.Sprintf("%s/%s/permissions", fronteggRolePath, id), headers, in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggRoleDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggRoleRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	headers := getTenantIdHeaders(d)
	clientHolder := meta.(*restclient.ClientHolder)
	var out []fronteggRole
	if err := clientHolder.ApiClient.GetWithHeaders(ctx, fronteggRolePath, headers, &out); err != nil {
		return diag.FromErr(err)
	}
	for _, c := range out {
		if c.ID == d.Id() {
			if err := resourceFronteggRoleDeserialize(d, c); err != nil {
				return diag.FromErr(err)
			}
			return diag.Diagnostics{}
		}
	}
	d.SetId("")
	return nil
}

func resourceFronteggRoleUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	headers := getTenantIdHeaders(d)
	clientHolder := meta.(*restclient.ClientHolder)
	{
		in := resourceFronteggRoleSerialize(d)
		if err := clientHolder.ApiClient.PatchWithHeaders(ctx, fmt.Sprintf("%s/%s", fronteggRolePath, d.Id()), headers, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	var out fronteggRole
	in := resourceFronteggRolePermissionsSerialize(d)
	if err := clientHolder.ApiClient.PutWithHeaders(ctx, fmt.Sprintf("%s/%s/permissions", fronteggRolePath, d.Id()), headers, in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggRoleDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggRoleDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	headers := getTenantIdHeaders(d)
	clientHolder := meta.(*restclient.ClientHolder)
	if err := clientHolder.ApiClient.DeleteWithHeaders(ctx, fmt.Sprintf("%s/%s", fronteggRolePath, d.Id()), headers, nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
