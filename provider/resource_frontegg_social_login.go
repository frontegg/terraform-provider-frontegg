package provider

import (
	"context"
	"fmt"
	"log"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceFronteggSocialLogin() *schema.Resource {
	return &schema.Resource{
		Description: `Configures social login for a specific provider.

Supported providers are: facebook, github, google, microsoft.`,

		CreateContext: resourceFronteggSocialLoginCreate,
		ReadContext:   resourceFronteggSocialLoginRead,
		UpdateContext: resourceFronteggSocialLoginUpdate,
		DeleteContext: resourceFronteggSocialLoginDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"provider_name": {
				Description:  "The social login provider name. Must be one of: facebook, github, google, microsoft.",
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice([]string{"facebook", "github", "google", "microsoft"}, false),
			},
			"client_id": {
				Description: "The client ID of the social login application to authenticate with. Required when setting **`customised`** parameter to true.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"redirect_url": {
				Description: "The URL to redirect to after a successful authentication.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"secret": {
				Description: "The secret associated with the social login application. Required when setting **`customised`** parameter to true.",
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
			},
			"customised": {
				Description: "Determine whether the SSO should use customized secret and client ID. When passing true, clientId and secret are also required.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
			},
			"additional_scopes": {
				Description: "Determine whether to ask for additional scopes when authenticating with the SSO provider.",
				Type:        schema.TypeSet,
				Optional:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func resourceFronteggSocialLoginSerialize(d *schema.ResourceData) fronteggSSO {
	sso := fronteggSSO{
		ClientID:    d.Get("client_id").(string),
		RedirectURL: d.Get("redirect_url").(string),
		Secret:      d.Get("secret").(string),
		Cusomised:   d.Get("customised").(bool),
		Type:        d.Get("provider_name").(string),
	}

	if v, ok := d.GetOk("additional_scopes"); ok {
		sso.AdditionalScopes = stringSetToList(v.(*schema.Set))
	}

	return sso
}

func resourceFronteggSocialLoginDeserialize(d *schema.ResourceData, f fronteggSSO, providerName string) error {
	d.SetId(providerName)

	if err := d.Set("provider_name", providerName); err != nil {
		return err
	}
	if err := d.Set("client_id", f.ClientID); err != nil {
		return err
	}
	if err := d.Set("redirect_url", f.RedirectURL); err != nil {
		return err
	}
	if err := d.Set("secret", f.Secret); err != nil {
		return err
	}
	if err := d.Set("customised", f.Cusomised); err != nil {
		return err
	}
	if err := d.Set("additional_scopes", f.AdditionalScopes); err != nil {
		return err
	}
	return nil
}

func resourceFronteggSocialLoginCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceFronteggSocialLoginUpdate(ctx, d, meta)
}

func resourceFronteggSocialLoginRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	providerName := d.Get("provider_name").(string)

	var out fronteggSSO
	clientHolder.ApiClient.Ignore404()
	if err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s", fronteggSSOURL, providerName), &out); err != nil {
		return diag.FromErr(err)
	}

	// If the provider is not active, the resource doesn't exist
	if !out.Active {
		d.SetId("")
		return nil
	}

	if err := resourceFronteggSocialLoginDeserialize(d, out, providerName); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggSocialLoginUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	providerName := d.Get("provider_name").(string)

	in := resourceFronteggSocialLoginSerialize(d)
	if err := clientHolder.ApiClient.Post(ctx, fronteggSSOURL, in, nil); err != nil {
		return diag.FromErr(err)
	}

	// Activate the provider
	if err := clientHolder.ApiClient.Post(ctx, fmt.Sprintf("%s/%s/activate", fronteggSSOURL, providerName), nil, nil); err != nil {
		return diag.FromErr(err)
	}

	return resourceFronteggSocialLoginRead(ctx, d, meta)
}

func resourceFronteggSocialLoginDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	providerName := d.Get("provider_name").(string)

	clientHolder.ApiClient.Ignore404()
	if err := clientHolder.ApiClient.Post(ctx, fmt.Sprintf("%s/%s/deactivate", fronteggSSOURL, providerName), nil, nil); err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Deactivated social login for provider: %s", providerName)
	return nil
}
