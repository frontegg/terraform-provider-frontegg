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

type fronteggSuperUser struct {
    SuperUser bool `json:"superUser"`
}

type fronteggUserRole struct {
	Id  string `json:"id,omitempty"`
	Key string `json:"key,omitempty"`
}

type fronteggUser struct {
	Key             string             `json:"id,omitempty"`
	Email           string             `json:"email,omitempty"`
	Password        string             `json:"password,omitempty"`
	CreateRoleIDs   []interface{}      `json:"roleIds,omitempty"`
	ReadRoleIDs     []fronteggUserRole `json:"roles,omitempty"`
	SkipInviteEmail bool               `json:"skipInviteEmail,omitempty"`
	Verified        bool               `json:"verified,omitempty"`
	SuperUser       bool               `json:"superUser,omitempty"`
}

const fronteggUserPath = "/identity/resources/users/v2"
const fronteggUserPathV1 = "/identity/resources/users/v1"

func resourceFronteggUser() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg user.`,

		CreateContext: resourceFronteggUserCreate,
		ReadContext:   resourceFronteggUserRead,
		DeleteContext: resourceFronteggUserDelete,
		UpdateContext: resourceFronteggUserUpdate,
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
			"skip_invite_email": {
				Description: "Skip sending the invite email. If true, user is automatically verified on creation.",
				Type:        schema.TypeBool,
				Optional:    true,
			},
			"automatically_verify": {
				Description: "Whether the user gets verified upon creation.",
				Type:        schema.TypeBool,
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
			"tenant_id": {
				Description: "The tenant ID for this user.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"superuser": {
			    Description: "Whether the user is a super user.",
			    Type:        schema.TypeBool,
			    Optional:    true,
			},
		},
	}
}

func resourceFronteggUserSerialize(d *schema.ResourceData) fronteggUser {
	log.Printf("role IDs: %#v", d.Get("role_ids").(*schema.Set).List())
	return fronteggUser{
		Email:           d.Get("email").(string),
		Password:        d.Get("password").(string),
		SkipInviteEmail: d.Get("skip_invite_email").(bool),
		CreateRoleIDs:   d.Get("role_ids").(*schema.Set).List(),
		SuperUser:       d.Get("superuser").(bool),
	}
}

func resourceFronteggUserDeserialize(d *schema.ResourceData, f fronteggUser) error {
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

func resourceFronteggUserCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggUserSerialize(d)
	var out fronteggUser
	headers := http.Header{}
	headers.Add("frontegg-tenant-id", d.Get("tenant_id").(string))
	if err := clientHolder.ApiClient.RequestWithHeaders(ctx, "POST", fronteggUserPath, headers, in, &out); err != nil {
		return diag.FromErr(err)
	}

	if err := resourceFronteggUserDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}

    superUser := d.Get("superuser").(bool)
    if superUser == true {
        in := fronteggSuperUser {
        SuperUser: superUser,
        }
        if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s/superuser", fronteggUserPathV1, out.Key), in, nil); err != nil {
            return diag.FromErr(err)
        }
    }

	if !d.Get("automatically_verify").(bool) {
		return nil
	}
	if err := clientHolder.ApiClient.Post(ctx, fmt.Sprintf("%s/%s/verify", fronteggUserPathV1, out.Key), nil, nil); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggUserRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	client := clientHolder.ApiClient
	client.Ignore404()
	var out fronteggUser
	headers := http.Header{}
	headers.Add("frontegg-tenant-id", d.Get("tenant_id").(string))
	if err := client.RequestWithHeaders(ctx, "GET", fmt.Sprintf("%s/%s", fronteggUserPathV1, d.Id()), headers, nil, &out); err != nil {
		return diag.FromErr(err)
	}
	if out.Key == "" {
		d.SetId("")
		return nil
	}

	if err := resourceFronteggUserDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggUserDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggUserPathV1, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggUserUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	// TODO: fields like phone number and avatar URL need https://docs.frontegg.com/reference/userscontrollerv1_updateuser

	// Email address:
	if d.HasChange("email") {
		email := d.Get("email").(string)
		if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s/email", fronteggUserPathV1, d.Id()), struct {
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

		if err := clientHolder.ApiClient.RequestWithHeaders(ctx, "POST", fmt.Sprintf("%s/passwords/change", fronteggUserPathV1), headers, struct {
			OldPW string `json:"password"`
			NewPW string `json:"newPassword"`
		}{oldPw, newPw}, nil); err != nil {
			return diag.FromErr(err)
		}

		if err := d.Set("password", newPw); err != nil {
			return diag.FromErr(err)
		}
	}

	// Super User:
	if d.HasChange("superuser") {
	    superUser := d.Get("superuser").(bool)
	    in := fronteggSuperUser {
	        SuperUser: superUser,
        }
        if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s/superuser", fronteggUserPathV1, d.Id()), in, nil); err != nil {
            return diag.FromErr(err)
        }
    }

	// Roles:
	if d.HasChange("role_ids") {
		headers := http.Header{}
		headers.Add("frontegg-tenant-id", d.Get("tenant_id").(string))

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
