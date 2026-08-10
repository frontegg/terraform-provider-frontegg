package provider

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceFronteggPortalUser() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg portal user.

Import this resource using the ` + "`tenant_id:user_id`" + ` format, since the
tenant cannot be recovered from the user ID alone.`,

		CreateContext: resourceFronteggPortalUserCreate,
		ReadContext:   resourceFronteggPortalUserRead,
		DeleteContext: resourceFronteggPortalUserDelete,
		UpdateContext: resourceFronteggPortalUserUpdate,
		Importer: &schema.ResourceImporter{
			StateContext: resourceFronteggPortalUserImport,
		},

		Schema: map[string]*schema.Schema{
			"tenant_id": {
				Description: "The ID of the tenant that the user belongs to.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"email": {
				Description: "The user's email address.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"password": {
				Description: "The user's login password. This field is write-only and will not be stored in state. Changes to this field are ignored after creation.",
				Type:        schema.TypeString,
				Sensitive:   true,
				Optional:    true,
				DiffSuppressFunc: func(k, old, newVal string, d *schema.ResourceData) bool {
					// Always suppress diff for password after resource creation
					return d.Id() != ""
				},
			},
			"role_ids": {
				Description: "List of the role IDs that the user has in the tenant",
				Type:        schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				MinItems: 1,
				Required: true,
			},
		},
	}
}

// The identity routes are tenant-scoped, so tenant_id has to survive an
// import. It is not part of the API response, hence the compound import ID.
func resourceFronteggPortalUserImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	parts := strings.SplitN(d.Id(), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid import format, expected tenant_id:user_id, got: %s", d.Id())
	}
	if err := d.Set("tenant_id", parts[0]); err != nil {
		return nil, err
	}
	d.SetId(parts[1])
	return []*schema.ResourceData{d}, nil
}

func resourceFronteggPortalUserHeaders(d *schema.ResourceData) http.Header {
	headers := http.Header{}
	headers.Add("frontegg-tenant-id", d.Get("tenant_id").(string))
	return headers
}

func resourceFronteggPortalUserSerialize(d *schema.ResourceData) fronteggUser {
	log.Printf("role IDs: %#v", d.Get("role_ids").(*schema.Set).List())
	return fronteggUser{
		Email:         d.Get("email").(string),
		Password:      d.Get("password").(string),
		CreateRoleIDs: d.Get("role_ids").(*schema.Set).List(),
	}
}

func resourceFronteggPortalUserDeserialize(d *schema.ResourceData, f fronteggUser) error {
	d.SetId(f.Key)
	if err := d.Set("email", f.Email); err != nil {
		return err
	}
	var roleIDs []string
	for _, roleID := range f.ReadRoleIDs {
		roleIDs = append(roleIDs, roleID.Id)
	}
	if err := d.Set("role_ids", roleIDs); err != nil {
		return err
	}
	return nil
}

func resourceFronteggPortalUserCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggPortalUserSerialize(d)
	var out fronteggUser
	if err := clientHolder.ApiClient.RequestWithHeaders(ctx, "POST", fronteggUserPath, resourceFronteggPortalUserHeaders(d), in, &out); err != nil {
		return diag.FromErr(fronteggUserApplicationError(err))
	}

	if err := resourceFronteggPortalUserDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggPortalUserRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	var out fronteggUser
	err := clientHolder.ApiClient.RequestWithHeaders(ctx, "GET", fmt.Sprintf("%s/%s", fronteggUserPathV1, d.Id()), resourceFronteggPortalUserHeaders(d), nil, &out)
	if restclient.IsNotFound(err) {
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.FromErr(err)
	}
	if out.Key == "" {
		d.SetId("")
		return nil
	}

	if err := resourceFronteggPortalUserDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggPortalUserDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	err := clientHolder.ApiClient.DeleteWithHeaders(ctx, fmt.Sprintf("%s/%s", fronteggUserPathV1, d.Id()), resourceFronteggPortalUserHeaders(d), nil)
	if err != nil && !restclient.IsNotFound(err) {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggPortalUserUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	// Email address:
	if d.HasChange("email") {
		email := d.Get("email").(string)
		// The email route is vendor-level and rejects a tenant header with
		// "Tenant ID is defined - forbidden route for tenants".
		if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s/email", fronteggUserPathV1, d.Id()), struct {
			Email string `json:"email"`
		}{email}, nil); err != nil {
			return diag.FromErr(err)
		}

		if err := d.Set("email", email); err != nil {
			return diag.FromErr(err)
		}
	}

	// Roles:
	if d.HasChange("role_ids") {
		headers := resourceFronteggPortalUserHeaders(d)

		oldsI, newsI := d.GetChange("role_ids")
		olds := oldsI.(*schema.Set)
		news := newsI.(*schema.Set)

		toAddSet := news.Difference(olds)
		toDelSet := olds.Difference(news)

		var toAdd, toDel []string

		for _, add := range toAddSet.List() {
			toAdd = append(toAdd, add.(string))
		}
		for _, del := range toDelSet.List() {
			toDel = append(toDel, del.(string))
		}

		if len(toAdd) > 0 {
			if err := clientHolder.ApiClient.RequestWithHeaders(ctx, "POST", fmt.Sprintf("%s/%s/roles", fronteggUserPathV1, d.Id()), headers, struct {
				RoleIds []string `json:"roleIds"`
			}{toAdd}, nil); err != nil {
				return diag.FromErr(err)
			}
		}
		if len(toDel) > 0 {
			if err := clientHolder.ApiClient.RequestWithHeaders(ctx, "DELETE", fmt.Sprintf("%s/%s/roles", fronteggUserPathV1, d.Id()), headers, struct {
				RoleIds []string `json:"roleIds"`
			}{toDel}, nil); err != nil {
				return diag.FromErr(err)
			}
		}

		if err := d.Set("role_ids", news); err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(d.Id())
	return nil

}
