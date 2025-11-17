package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/frontegg/terraform-provider-frontegg/provider/validators"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	fronteggEmailPorivderPath   = "/identity/resources/mail/v2/configurations"
	fronteggEmailPorivderPathV1 = "/identity/resources/mail/v1/configurations"
)

type fronteggEmailProviderPayload struct {
	Id       string `json:"id,omitempty"`
	Region   string `json:"region,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Secret   string `json:"secret,omitempty"`
	Provider string `json:"provider,omitempty"`
}

type fronteggEmailProvider struct {
	Payload fronteggEmailProviderPayload `json:"payload,omitempty"`
}

type Extension struct {
	ExtensionName  string `json:"extensionName,omitempty"`
	ExtensionValue string `json:"extensionValue,omitempty"`
}

type fronteggEmailProviderResponse struct {
	Secret    string      `json:"secret,omitempty"`
	CreatedAt string      `json:"createdAt,omitempty"`
	UpdatedAt string      `json:"updatedAt,omitempty"`
	Extension []Extension `json:"extension,omitempty"`
}

func resourceFronteggEmailProvider() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg Email provider.`,

		CreateContext: resourceFronteggEmailProviderCreate,
		ReadContext:   resourceFronteggEmailProviderRead,
		UpdateContext: resourceFronteggEmailProviderUpdate,
		DeleteContext: resourceFronteggEmailProviderDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"provider_id": {
				Description: "Provider ID (required only for AWS SES).",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"region": {
				Description: "Required for AWS SES or Mailgun.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"domain": {
				Description: "Required for Mailgun (required only for Mailgun).",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"secret": {
				Description: "A secret to be included with the event.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"provider_name": {
				Description:  "Name of the email provider (If the provider is changed, the old provider's configuration will be deleted).",
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validators.ValidateProvider,
			},
			"created_at": {
				Description: "The timestamp at which the permission was created.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"updated_at": {
				Description: "The timestamp at which the permission was updated.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
		CustomizeDiff: validators.ValidateRequiredFields,
	}

}

func resourceFronteggEmailProviderSerialize(d *schema.ResourceData) fronteggEmailProvider {
	return fronteggEmailProvider{
		Payload: fronteggEmailProviderPayload{
			Secret:   d.Get("secret").(string),
			Provider: d.Get("provider_name").(string),
			Id:       d.Get("provider_id").(string),
			Region:   d.Get("region").(string),
			Domain:   d.Get("domain").(string),
		},
	}
}

func resourceFronteggEmailProviderDeserialize(d *schema.ResourceData, f *fronteggEmailProviderResponse) error {

	if err := d.Set("provider_name", d.Id()); err != nil {
		return err
	}

	fields := map[string]string{
		"secret":     f.Secret,
		"updated_at": f.UpdatedAt,
		"created_at": f.CreatedAt,
	}

	for key, value := range fields {
		if err := d.Set(key, value); err != nil {
			return fmt.Errorf("error setting %s: %s", key, err)
		}
	}

	return nil
}

func resourceFronteggEmailProviderCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggEmailProviderSerialize(d)
	provider := d.Get("provider_name").(string)
	d.SetId(provider)
	if err := clientHolder.ApiClient.Post(ctx, fronteggEmailPorivderPath, in, nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggEmailProviderRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	var out fronteggEmailProviderResponse
	clientHolder.ApiClient.Ignore404()
	if err := clientHolder.ApiClient.Get(ctx, fronteggEmailPorivderPathV1, &out); err != nil {
		return diag.FromErr(err)
	}

	// If no email provider configuration exists, the response will be empty
	if out.Secret == "" && out.CreatedAt == "" {
		d.SetId("")
		return nil
	}

	if err := resourceFronteggEmailProviderDeserialize(d, &out); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggEmailProviderUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggEmailProviderSerialize(d)
	providerName := "provider_name"

	provider := d.Get(providerName).(string)
	d.SetId(provider)

	if d.HasChange(providerName) {
		err := clientHolder.ApiClient.Delete(ctx, fronteggEmailPorivderPathV1, nil)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if err := clientHolder.ApiClient.Post(ctx, fronteggEmailPorivderPath, in, nil); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggEmailProviderDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	err := clientHolder.ApiClient.Delete(ctx, fronteggEmailPorivderPathV1, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
