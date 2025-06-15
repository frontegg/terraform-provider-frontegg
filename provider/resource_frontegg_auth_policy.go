package provider

import (
	"context"
	"strings"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggAuthPolicyURL = "/identity/resources/configurations/v1"

type fronteggAuthPolicy struct {
	ID                            string `json:"id"`
	AllowNotVerifiedUsersLogin    bool   `json:"allowNotVerifiedUsersLogin"`
	AllowSignups                  bool   `json:"allowSignups"`
	AllowTenantInvitations        bool   `json:"allowTenantInvitations"`
	APITokensEnabled              bool   `json:"apiTokensEnabled"`
	CookieSameSite                string `json:"cookieSameSite"`
	DefaultRefreshTokenExpiration int    `json:"defaultRefreshTokenExpiration"`
	DefaultTokenExpiration        int    `json:"defaultTokenExpiration"`
	ForcePermissions              bool   `json:"forcePermissions"`
	JWTAlgorithm                  string `json:"jwtAlgorithm"`
	PublicKey                     string `json:"publicKey"`
	AuthStrategy                  string `json:"authStrategy"`
	MachineToMachineAuthStrategy  string `json:"machineToMachineAuthStrategy"`
}

func resourceFronteggAuthPolicy() *schema.Resource {
	return &schema.Resource{
		Description: `Configures the general authentication policy for the workspace.

This is a singleton resource. You must only create one frontegg_auth_policy resource
per Frontegg provider.

**Note:** This resource cannot be deleted. When destroyed, Terraform will remove it from the state file, but the authentication policy will remain in its last-applied state.`,

		CreateContext: resourceFronteggAuthPolicyCreate,
		ReadContext:   resourceFronteggAuthPolicyRead,
		UpdateContext: resourceFronteggAuthPolicyUpdate,
		DeleteContext: resourceFronteggAuthPolicyDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"allow_unverified_users": {
				Description: "Whether unverified users are allowed to log in.",
				Type:        schema.TypeBool,
				Required:    true,
			},
			"allow_signups": {
				Description: "Whether users are allowed to sign up.",
				Type:        schema.TypeBool,
				Required:    true,
			},
			"allow_tenant_invitations": {
				Description: "Allow tenants to invite new users via an invitation link.",
				Type:        schema.TypeBool,
				Required:    true,
			},
			"enable_api_tokens": {
				Description: "Whether users can create API tokens.",
				Type:        schema.TypeBool,
				Required:    true,
			},
			"machine_to_machine_auth_strategy": {
				Description: `Type of tokens users will be able to generate.
				Must be one of "ClientCredentials" or "AccessToken".`,
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "ClientCredentials",
				ValidateFunc: validation.StringInSlice([]string{"ClientCredentials", "AccessToken"}, false),
			},
			"enable_roles": {
				Description: "Whether granular roles and permissions are enabled.",
				Type:        schema.TypeBool,
				Required:    true,
			},
			"jwt_algorithm": {
				Description:  "The algorithm Frontegg uses to sign JWT tokens.",
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "RS256",
				ValidateFunc: validation.StringInSlice([]string{"RS256"}, false),
			},
			"jwt_access_token_expiration": {
				Description: "The expiration time for the JWT access tokens issued by Frontegg.",
				Type:        schema.TypeInt,
				Required:    true,
			},
			"jwt_refresh_token_expiration": {
				Description: "The expiration time for the JWT refresh tokens issued by Frontegg.",
				Type:        schema.TypeInt,
				Required:    true,
			},
			"jwt_public_key": {
				Description: "The public key that Frontegg uses to sign JWT tokens.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"same_site_cookie_policy": {
				Description: `The SameSite policy to use for Frontegg cookies.

Must be one of "none", "lax", or "strict".`,
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{"none", "lax", "strict"}, false),
			},
			"auth_strategy": {
				Description: `The authentication strategy to use for people logging in.

Must be one of "EmailAndPassword", "Code", "MagicLink", "NoLocalAuthentication", "SmsCode"`,
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{"EmailAndPassword", "Code", "MagicLink", "NoLocalAuthentication", "SmsCode"}, false),
			},
		},
	}
}

func resourceFronteggAuthPolicySerialize(d *schema.ResourceData) fronteggAuthPolicy {
	return fronteggAuthPolicy{
		AllowNotVerifiedUsersLogin:    d.Get("allow_unverified_users").(bool),
		AllowSignups:                  d.Get("allow_signups").(bool),
		AllowTenantInvitations:        d.Get("allow_tenant_invitations").(bool),
		APITokensEnabled:              d.Get("enable_api_tokens").(bool),
		MachineToMachineAuthStrategy:  d.Get("machine_to_machine_auth_strategy").(string),
		ForcePermissions:              d.Get("enable_roles").(bool),
		JWTAlgorithm:                  d.Get("jwt_algorithm").(string),
		DefaultTokenExpiration:        d.Get("jwt_access_token_expiration").(int),
		DefaultRefreshTokenExpiration: d.Get("jwt_refresh_token_expiration").(int),
		PublicKey:                     d.Get("jwt_public_key").(string),
		CookieSameSite:                strings.ToUpper(d.Get("same_site_cookie_policy").(string)),
		AuthStrategy:                  d.Get("auth_strategy").(string),
	}
}

func resourceFronteggAuthPolicyDeserialize(d *schema.ResourceData, f fronteggAuthPolicy) error {
	d.SetId(f.ID)
	if err := d.Set("allow_unverified_users", f.AllowNotVerifiedUsersLogin); err != nil {
		return err
	}
	if err := d.Set("allow_signups", f.AllowSignups); err != nil {
		return err
	}
	if err := d.Set("allow_tenant_invitations", f.AllowTenantInvitations); err != nil {
		return err
	}
	if err := d.Set("enable_api_tokens", f.APITokensEnabled); err != nil {
		return err
	}
	if err := d.Set("machine_to_machine_auth_strategy", f.MachineToMachineAuthStrategy); err != nil {
		return err
	}
	if err := d.Set("enable_roles", f.ForcePermissions); err != nil {
		return err
	}
	if err := d.Set("jwt_algorithm", f.JWTAlgorithm); err != nil {
		return err
	}
	if err := d.Set("jwt_access_token_expiration", f.DefaultTokenExpiration); err != nil {
		return err
	}
	if err := d.Set("jwt_refresh_token_expiration", f.DefaultRefreshTokenExpiration); err != nil {
		return err
	}
	if err := d.Set("jwt_public_key", f.PublicKey); err != nil {
		return err
	}
	if err := d.Set("same_site_cookie_policy", strings.ToLower(f.CookieSameSite)); err != nil {
		return err
	}
	if err := d.Set("auth_strategy", f.AuthStrategy); err != nil {
		return err
	}
	return nil
}

func resourceFronteggAuthPolicyCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceFronteggAuthPolicyUpdate(ctx, d, meta)
}

func resourceFronteggAuthPolicyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	var out fronteggAuthPolicy
	if err := clientHolder.ApiClient.Get(ctx, fronteggAuthPolicyURL, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggAuthPolicyDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggAuthPolicyUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggAuthPolicySerialize(d)
	if err := clientHolder.ApiClient.Post(ctx, fronteggAuthPolicyURL, in, nil); err != nil {
		return diag.FromErr(err)
	}
	return resourceFronteggAuthPolicyRead(ctx, d, meta)
}

func resourceFronteggAuthPolicyDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// Auth policy is a configuration that cannot be deleted, only reset to defaults
	// We'll leave it in its current state and just remove from Terraform state
	return nil
}
