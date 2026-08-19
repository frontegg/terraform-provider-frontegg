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
	// The API keeps and returns credentials even for a provider that is not
	// using customised ones. Writing those into state produces permanent drift
	// against a configuration that deliberately omits them.
	clientID, secret := f.ClientID, f.Secret
	if !f.Cusomised {
		clientID, secret = "", ""
	}
	if err := d.Set("client_id", clientID); err != nil {
		return err
	}
	if err := d.Set("redirect_url", f.RedirectURL); err != nil {
		return err
	}
	if err := d.Set("secret", secret); err != nil {
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

// socialLoginAdoptionReason reports why creating over an existing provider
// configuration would destroy information Terraform did not supply, or "" when
// it is safe to proceed.
//
// Creating a provider is a POST that overwrites whatever is already there, so
// pointing this resource at a provider somebody configured by hand silently
// takes it over — and applying without credentials erases the ones already
// stored, unrecoverably in the case of the secret.
//
// Deleting only deactivates a provider and leaves its credentials behind, so
// neither check may fire on the configuration this resource itself left
// behind: after a destroy the provider is inactive, and a customised resource
// re-supplies the same client ID it had before.
func socialLoginAdoptionReason(existing fronteggSSO, configuredClientID string) string {
	if existing.Active {
		return "it is already active"
	}
	if existing.ClientID != "" && configuredClientID == "" {
		return "it already has credentials, which this configuration does not set and would erase"
	}
	return ""
}

func resourceFronteggSocialLoginCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	providerName := d.Get("provider_name").(string)

	var existing fronteggSSO
	err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s", fronteggSSOURL, providerName), &existing)
	switch {
	case restclient.IsNotFound(err):
		// Nothing configured for this provider; safe to create.
	case err != nil:
		return diag.FromErr(err)
	default:
		if reason := socialLoginAdoptionReason(existing, d.Get("client_id").(string)); reason != "" {
			return diag.Errorf(
				"social login provider %q cannot be created because %s.\n\n"+
					"Terraform will not take over a provider configuration it did not create, "+
					"because applying over one overwrites its settings and can erase credentials "+
					"that cannot be recovered. To manage the existing configuration, import it:\n\n"+
					"    terraform import <resource address> %s\n\n"+
					"Otherwise remove the provider configuration in Frontegg first.",
				providerName, reason, providerName,
			)
		}
	}

	return resourceFronteggSocialLoginUpdate(ctx, d, meta)
}

func resourceFronteggSocialLoginRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	// On import only the ID is set, and the ID is the provider name.
	providerName := d.Get("provider_name").(string)
	if providerName == "" {
		providerName = d.Id()
	}

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
