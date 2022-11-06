package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const fronteggRedirectUriPath = "/oauth/resources/configurations/v1/redirect-uri"

type fronteggRedirectUri struct {
	RedirectUri string `json:"redirectUri,omitempty"`
	Key         string `json:"id,omitempty"`
}

func resourceFronteggRedirectUri() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg Redirect URI.`,

		CreateContext: resourceFronteggRedirectUriCreate,
		ReadContext:   resourceFronteggRedirectUriRead,
		DeleteContext: resourceFronteggRedirectUriDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"redirect_uri": {
				Description: "The redirect URI.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"key": {
				Description: "The redirect URI key.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func resourceFronteggRedirectUriSerialize(d *schema.ResourceData) fronteggRedirectUri {
	return fronteggRedirectUri{
		Key:         d.Get("key").(string),
		RedirectUri: d.Get("redirect_uri").(string),
	}
}

func resourceFronteggRedirectUriDeserialize(d *schema.ResourceData, f fronteggRedirectUri) error {
	d.SetId(f.Key)
	if err := d.Set("key", f.Key); err != nil {
		return err
	}
	if err := d.Set("redirect_uri", f.RedirectUri); err != nil {
		return err
	}
	return nil
}

func resourceFronteggRedirectUriCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggRedirectUriSerialize(d)
	if err := clientHolder.ApiClient.Post(ctx, fronteggRedirectUriPath, in, nil); err != nil {
		return diag.FromErr(err)
	}
	var out struct {
		RedirectURIs []fronteggRedirectUri `json:"redirectUris"`
	}
	if err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s", fronteggRedirectUriPath), &out); err != nil {
		return diag.FromErr(err)
	}
	for _, c := range out.RedirectURIs {
		if c.RedirectUri == in.RedirectUri || c.RedirectUri == fmt.Sprintf("%s/", in.RedirectUri) {
			if err := resourceFronteggRedirectUriDeserialize(d, c); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}
	return nil
}

func resourceFronteggRedirectUriRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	var out struct {
		RedirectURIs []fronteggRedirectUri `json:"redirectUris"`
	}
	if err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s", fronteggRedirectUriPath), &out); err != nil {
		return diag.FromErr(err)
	}
	for _, c := range out.RedirectURIs {
		if c.Key == d.Id() {
			if err := resourceFronteggRedirectUriDeserialize(d, c); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}
	d.SetId("")
	return nil
}

func resourceFronteggRedirectUriDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggRedirectUriPath, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
