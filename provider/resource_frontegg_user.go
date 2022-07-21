package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type fronteggUser struct {
	Key             string   `json:"id,omitempty"`
	Email           string   `json:"email,omitempty"`
	Password        string   `json:"password,omitempty"`
	Roles           []string `json:"roles,omitempty"`
	SkipInviteEmail bool     `json:"skipInviteEmail,omitempty"`
	Verified        bool     `json:"verified,omitempty"`
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
			"roles": {
				Description: "List of the roles that the user has in the tenant",
				Type:        schema.TypeList,
				Elem:        schema.TypeString,
				MinItems:    1,
				Required:    true,
			},
			"key": {
				Description: "A human-readable identifier for the user.",
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
			},
			"tenant_id": {
				Description: "The tenant ID for this user.",
				Type:        schema.TypeString,
				Required:    true,
			},
		},
	}
}

func resourceFronteggUserSerialize(d *schema.ResourceData) fronteggUser {
	return fronteggUser{
		Email:           d.Get("email").(string),
		Password:        d.Get("password").(string),
		SkipInviteEmail: d.Get("skip_invite_email").(bool),
		Roles:           d.Get("roles").([]string),
	}
}

func resourceFronteggUserDeserialize(d *schema.ResourceData, f fronteggUser) error {
	d.SetId(f.Key)
	if err := d.Set("email", f.Email); err != nil {
		return err
	}
	if err := d.Set("key", f.Key); err != nil {
		return err
	}
	if err := d.Set("roles", f.Roles); err != nil {
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

	if !d.Get("automatically_verify").(bool) {
		return nil
	}
	if err := clientHolder.ApiClient.RequestWithHeaders(ctx, "POST", fmt.Sprintf("%s/%s/verify", fronteggUserPathV1, out.Key), headers, in, &out); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggUserRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	var out []fronteggUser
	if err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s", fronteggUserPathV1, d.Id()), &out); err != nil {
		return diag.FromErr(err)
	}
	for _, c := range out {
		if c.Key == d.Id() {
			if err := resourceFronteggUserDeserialize(d, c); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}
	d.SetId("")
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
	var out fronteggUser
	in := resourceFronteggUserSerialize(d)
	headers := http.Header{}
	headers.Add("frontegg-tenant-id", d.Get("tenant_id").(string))
	headers.Add("frontegg-user-id", d.Get("key").(string))
	if err := clientHolder.ApiClient.RequestWithHeaders(ctx, "PUT", fronteggUserPathV1, headers, in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggUserDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil

}
