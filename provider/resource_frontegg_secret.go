package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const fronteggSecretPath = "/custom-code/resources/secrets/v1"

type fronteggSecret struct {
	ID    string `json:"id,omitempty"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

func resourceFronteggSecret() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg secret.`,

		CreateContext: resourceFronteggSecretCreate,
		ReadContext:   resourceFronteggSecretRead,
		UpdateContext: resourceFronteggSecretUpdate,
		DeleteContext: resourceFronteggSecretDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"key": {
				Description: "The key of the secret.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"value": {
				Description: "The value of the secret.",
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
			},
		},
	}
}

func resourceFronteggSecretSerialize(d *schema.ResourceData) fronteggSecret {
	return fronteggSecret{
		Key:   d.Get("key").(string),
		Value: d.Get("value").(string),
	}
}

func resourceFronteggSecretDeserialize(d *schema.ResourceData, f fronteggSecret) error {
	d.SetId(f.ID)
	if err := d.Set("key", f.Key); err != nil {
		return err
	}
	// Note: We don't set the value back from the API as it's not returned for security reasons
	return nil
}

func resourceFronteggSecretCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggSecretSerialize(d)
	var out fronteggSecret
	if err := clientHolder.ApiClient.Post(ctx, fronteggSecretPath, in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggSecretDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggSecretRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	var out struct {
		Items []fronteggSecret `json:"items"`
	}
	if err := clientHolder.ApiClient.Get(ctx, fronteggSecretPath, &out); err != nil {
		return diag.FromErr(err)
	}
	for _, s := range out.Items {
		if s.ID == d.Id() {
			if err := resourceFronteggSecretDeserialize(d, s); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}
	d.SetId("")
	return nil
}

func resourceFronteggSecretUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := fronteggSecret{
		Value: d.Get("value").(string),
	}
	if err := clientHolder.ApiClient.Patch(ctx, fmt.Sprintf("%s/%s", fronteggSecretPath, d.Id()), in, nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggSecretDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggSecretPath, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
