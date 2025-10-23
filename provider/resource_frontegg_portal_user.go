package provider

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceFronteggPortalUser() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg portal user.`,

		CreateContext: resourceFronteggPortalUserCreate,
		ReadContext:   resourceFronteggPortalUserRead,
		DeleteContext: resourceFronteggPortalUserDelete,
		UpdateContext: resourceFronteggPortalUserUpdate,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"email": {
				Description: "The user's email address.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"password": {
				Description: "The user's login password.",
				Type:        schema.TypeString,
				Sensitive:   true,
				Optional:    true,
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
	headers := http.Header{}
	if err := clientHolder.PortalClient.RequestWithHeaders(ctx, "POST", fronteggUserPath, headers, in, &out); err != nil {
		return diag.FromErr(err)
	}

	if err := resourceFronteggPortalUserDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggPortalUserRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	client := clientHolder.PortalClient
	client.Ignore404()
	var out fronteggUser
	headers := http.Header{}
	if err := client.RequestWithHeaders(ctx, "GET", fmt.Sprintf("%s/%s", fronteggUserPathV1, d.Id()), headers, nil, &out); err != nil {
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
	if err := clientHolder.PortalClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggUserPathV1, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggPortalUserUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	// TODO: fields like phone number and avatar URL need https://docs.frontegg.com/reference/userscontrollerv1_updateuser

	// Email address:
	if d.HasChange("email") {
		email := d.Get("email").(string)
		if err := clientHolder.PortalClient.Put(ctx, fmt.Sprintf("%s/%s/email", fronteggUserPathV1, d.Id()), struct {
			Email string `json:"email"`
		}{email}, nil); err != nil {
			return diag.FromErr(err)
		}

		if err := d.Set("email", email); err != nil {
			return diag.FromErr(err)
		}
	}

	// Password:
	if d.HasChange("password") {
		headers := http.Header{}
		headers.Add("frontegg-user-id", d.Id())

		oldI, newI := d.GetChange("password")
		oldPw := oldI.(string)
		newPw := newI.(string)

		if err := clientHolder.PortalClient.RequestWithHeaders(ctx, "POST", fmt.Sprintf("%s/passwords/change", fronteggUserPathV1), headers, struct {
			OldPW string `json:"password"`
			NewPW string `json:"newPassword"`
		}{oldPw, newPw}, nil); err != nil {
			return diag.FromErr(err)
		}

		if err := d.Set("password", newPw); err != nil {
			return diag.FromErr(err)
		}
	}

	// Roles:
	if d.HasChange("role_ids") {
		headers := http.Header{}

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
			if err := clientHolder.PortalClient.RequestWithHeaders(ctx, "POST", fmt.Sprintf("%s/%s/roles", fronteggUserPathV1, d.Id()), headers, struct {
				RoleIds []string `json:"roleIds"`
			}{toAdd}, nil); err != nil {
				return diag.FromErr(err)
			}
		}
		if len(toDel) > 0 {
			if err := clientHolder.PortalClient.RequestWithHeaders(ctx, "DELETE", fmt.Sprintf("%s/%s/roles", fronteggUserPathV1, d.Id()), headers, struct {
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
