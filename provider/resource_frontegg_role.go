package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const fronteggRolePath = "/identity/resources/roles/v1"

// Without frontegg-tenant-id, roles v2 lists the roles of every tenant, not only vendor-level ones.
const fronteggRoleV2Path = "/identity/resources/roles/v2"

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

type fronteggRoleTenant struct {
	TenantID *string `json:"tenantId"`
}

type fronteggRolePage struct {
	Items []fronteggRole `json:"items"`
}

func resourceFronteggRole() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg role.`,

		CreateContext: resourceFronteggRoleCreate,
		ReadContext:   resourceFronteggRoleRead,
		UpdateContext: resourceFronteggRoleUpdate,
		DeleteContext: resourceFronteggRoleDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceFronteggRoleImport,
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
				Description: `The ID of the tenant that owns the role. Leave unset for a vendor-level role available to all tenants.

Changing it moves the existing role in place: to the new tenant, which must exist and which unassigns the role from users of other tenants, or to vendor level when removed.`,
				Type:     schema.TypeString,
				Optional: true,
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

// resourceFronteggRoleImport accepts "role_id" for vendor roles and
// "tenant_id:role_id" for tenant roles, so that Read sends frontegg-tenant-id.
func resourceFronteggRoleImport(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	parts := strings.SplitN(d.Id(), ":", 2)
	if len(parts) == 1 {
		return []*schema.ResourceData{d}, nil
	}
	if parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid import format, expected role_id or tenant_id:role_id, got: %s", d.Id())
	}
	if err := d.Set("tenant_id", parts[0]); err != nil {
		return nil, err
	}
	d.SetId(parts[1])
	return []*schema.ResourceData{d}, nil
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
			if strings.Contains(err.Error(), "Roles already exists") {
				roleKey := d.Get("key").(string)
				tenantID := d.Get("tenant_id").(string)
				var existingRoles []fronteggRole
				if err := clientHolder.ApiClient.GetWithHeaders(ctx, fronteggRolePath, headers, &existingRoles); err != nil {
					return diag.FromErr(err)
				}

				// With a tenant header the list also holds vendor-level roles, which may share the key.
				var existingRole *fronteggRole
				for _, role := range existingRoles {
					if role.Key == roleKey && role.TenantID == tenantID {
						existingRole = &role
						break
					}
				}

				if existingRole == nil {
					return diag.Errorf("role with key '%s' not found after 'already exists' error", roleKey)
				}

				d.SetId(existingRole.ID)
				return resourceFronteggRoleUpdate(ctx, d, meta)
			}
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
	// The role is not in the tenant that state names: it was moved outside Terraform, or an
	// older provider saved a tenant_id it failed to move the role to. Record its real tenant
	// so the plan moves it back instead of creating a second role.
	role, err := findFronteggRoleInAnyTenant(ctx, clientHolder, d.Id(), d.Get("key").(string))
	if err != nil {
		return diag.FromErr(err)
	}
	if role == nil {
		d.SetId("")
		return nil
	}
	if err := resourceFronteggRoleDeserialize(d, *role); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

// findFronteggRoleInAnyTenant pages roles v2 for the role with the given ID. key narrows the
// search, since v2 filters by key or name but not by ID; an empty key pages every role.
func findFronteggRoleInAnyTenant(ctx context.Context, clientHolder *restclient.ClientHolder, id, key string) (*fronteggRole, error) {
	const limit = 100
	for offset := 0; ; offset += limit {
		q := url.Values{}
		q.Set("_sortBy", "createdAt")
		q.Set("_limit", strconv.Itoa(limit))
		q.Set("_offset", strconv.Itoa(offset))
		if key != "" {
			q.Set("_filter", key)
		}
		var page fronteggRolePage
		if err := clientHolder.ApiClient.Get(ctx, fronteggRoleV2Path+"?"+q.Encode(), &page); err != nil {
			return nil, err
		}
		for i := range page.Items {
			if page.Items[i].ID == id {
				return &page.Items[i], nil
			}
		}
		if len(page.Items) < limit {
			return nil, nil
		}
	}
}

func resourceFronteggRoleUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	headers := getTenantIdHeaders(d)
	clientHolder := meta.(*restclient.ClientHolder)
	// The tenant-scoped role routes only find roles of the tenant in the header, so move
	// the role first. Create reaches Update only for a role it found in the planned tenant.
	if d.HasChange("tenant_id") && !d.IsNewResource() {
		if diags := moveFronteggRole(ctx, clientHolder, d); diags != nil {
			// Nothing was changed yet, so keep the prior state; otherwise the next Read
			// would look for the role in a tenant it never reached.
			d.Partial(true)
			return diags
		}
	}
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

func moveFronteggRole(ctx context.Context, clientHolder *restclient.ClientHolder, d *schema.ResourceData) diag.Diagnostics {
	var in fronteggRoleTenant
	if tenantID := d.Get("tenant_id").(string); tenantID != "" {
		// The move endpoint accepts any tenant ID and unassigns the role from users of every
		// other tenant, so a typo would strip the role from all its users.
		exists, err := fronteggTenantExists(ctx, clientHolder, tenantID)
		if err != nil {
			return diag.FromErr(err)
		}
		if !exists {
			return diag.Errorf("tenant %q does not exist; role %s was not moved", tenantID, d.Id())
		}
		in.TenantID = &tenantID
	}
	if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s/tenant", fronteggRolePath, d.Id()), in, nil); err != nil {
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
