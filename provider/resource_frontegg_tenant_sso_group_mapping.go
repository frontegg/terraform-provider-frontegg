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

const fronteggSSOGroupPath = "/team/resources/sso/v1/configurations/%s/groups"

type fronteggSSOGroup struct {
	ID      string   `json:"id,omitempty"`
	Group   string   `json:"group"`
	RoleIDs []string `json:"roleIds,omitempty"`
}

func resourceFronteggTenantSSOGroupMapping() *schema.Resource {
	return &schema.Resource{
		Description: `Maps an IdP group to one or more Frontegg roles for a tenant SSO configuration. When a user authenticates via SSO and belongs to the specified IdP group, they are automatically assigned the mapped Frontegg roles.`,

		CreateContext: resourceFronteggTenantSSOGroupMappingCreate,
		ReadContext:   resourceFronteggTenantSSOGroupMappingRead,
		UpdateContext: resourceFronteggTenantSSOGroupMappingUpdate,
		DeleteContext: resourceFronteggTenantSSOGroupMappingDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceFronteggTenantSSOGroupMappingImport,
		},

		Schema: map[string]*schema.Schema{
			"tenant_id": {
				Description: "The ID of the tenant that owns the SSO group mapping.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"sso_config_id": {
				Description: "The ID of the SSO configuration to which this group mapping belongs. Can be the ID of a `frontegg_tenant_saml_config` or `frontegg_tenant_oidc_config` resource.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"group": {
				Description: "The name of the SSO group to map.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"role_ids": {
				Description: "The IDs of the Frontegg roles to assign to members of this IdP group. Use the `id` attribute of `frontegg_role` resources.",
				Type:        schema.TypeSet,
				Required:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceFronteggTenantSSOGroupMappingHeaders(d *schema.ResourceData) (http.Header, error) {
	tenantID := d.Get("tenant_id").(string)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required but is empty; use 'tenant_id:sso_config_id:group_id' format when importing")
	}
	headers := http.Header{}
	headers.Add("frontegg-tenant-id", tenantID)
	return headers, nil
}

func resourceFronteggTenantSSOGroupMappingSerialize(d *schema.ResourceData) fronteggSSOGroup {
	rawRoleIDs := d.Get("role_ids").(*schema.Set).List()
	roleIDs := make([]string, len(rawRoleIDs))
	for i, v := range rawRoleIDs {
		roleIDs[i] = v.(string)
	}
	return fronteggSSOGroup{
		Group:   d.Get("group").(string),
		RoleIDs: roleIDs,
	}
}

func resourceFronteggTenantSSOGroupMappingDeserialize(d *schema.ResourceData, f fronteggSSOGroup) error {
	d.SetId(f.ID)
	if err := d.Set("group", f.Group); err != nil {
		return err
	}
	if err := d.Set("role_ids", f.RoleIDs); err != nil {
		return err
	}
	return nil
}

func resourceFronteggTenantSSOGroupMappingCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	headers, err := resourceFronteggTenantSSOGroupMappingHeaders(d)
	if err != nil {
		return diag.FromErr(err)
	}
	ssoConfigID := d.Get("sso_config_id").(string)
	path := fmt.Sprintf(fronteggSSOGroupPath, ssoConfigID)

	in := resourceFronteggTenantSSOGroupMappingSerialize(d)
	var out fronteggSSOGroup
	if err := clientHolder.ApiClient.PostWithHeaders(ctx, path, headers, in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggTenantSSOGroupMappingDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggTenantSSOGroupMappingRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	headers, err := resourceFronteggTenantSSOGroupMappingHeaders(d)
	if err != nil {
		return diag.FromErr(err)
	}
	ssoConfigID := d.Get("sso_config_id").(string)
	path := fmt.Sprintf(fronteggSSOGroupPath, ssoConfigID)

	var out []fronteggSSOGroup
	if err := clientHolder.ApiClient.GetWithHeaders(ctx, path, headers, &out); err != nil {
		return diag.FromErr(err)
	}
	for _, g := range out {
		if g.ID == d.Id() {
			if err := resourceFronteggTenantSSOGroupMappingDeserialize(d, g); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}
	d.SetId("")
	return nil
}

func resourceFronteggTenantSSOGroupMappingUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	headers, err := resourceFronteggTenantSSOGroupMappingHeaders(d)
	if err != nil {
		return diag.FromErr(err)
	}
	ssoConfigID := d.Get("sso_config_id").(string)
	path := fmt.Sprintf("%s/%s", fmt.Sprintf(fronteggSSOGroupPath, ssoConfigID), d.Id())

	in := resourceFronteggTenantSSOGroupMappingSerialize(d)
	if err := clientHolder.ApiClient.PatchWithHeaders(ctx, path, headers, in, nil); err != nil {
		return diag.FromErr(err)
	}
	return resourceFronteggTenantSSOGroupMappingRead(ctx, d, meta)
}

func resourceFronteggTenantSSOGroupMappingDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	headers, err := resourceFronteggTenantSSOGroupMappingHeaders(d)
	if err != nil {
		return diag.FromErr(err)
	}
	ssoConfigID := d.Get("sso_config_id").(string)
	path := fmt.Sprintf("%s/%s", fmt.Sprintf(fronteggSSOGroupPath, ssoConfigID), d.Id())

	if err := clientHolder.ApiClient.DeleteWithHeaders(ctx, path, headers, nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggTenantSSOGroupMappingImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	parts := strings.SplitN(d.Id(), ":", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, fmt.Errorf("invalid import ID format, expected tenant_id:sso_config_id:group_id, got: %s", d.Id())
	}
	tenantID := parts[0]
	ssoConfigID := parts[1]
	groupID := parts[2]

	d.SetId(groupID)
	if err := d.Set("tenant_id", tenantID); err != nil {
		return nil, err
	}
	if err := d.Set("sso_config_id", ssoConfigID); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}
