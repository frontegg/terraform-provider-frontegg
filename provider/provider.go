package provider

import (
	"context"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func init() {
	schema.DescriptionKind = schema.StringMarkdown
}

func New(version string) func() *schema.Provider {
	return func() *schema.Provider {
		return &schema.Provider{
			Schema: map[string]*schema.Schema{
				"client_id": {
					Description: "The client ID for a Frontegg portal API key.",
					Type:        schema.TypeString,
					Required:    true,
					DefaultFunc: schema.EnvDefaultFunc("FRONTEGG_CLIENT_ID", nil),
				},
				"secret_key": {
					Description: "The corresponding secret key for the API key.",
					Type:        schema.TypeString,
					Required:    true,
					Sensitive:   true,
					DefaultFunc: schema.EnvDefaultFunc("FRONTEGG_SECRET_KEY", nil),
				},
			},
			DataSourcesMap: map[string]*schema.Resource{
				"frontegg_permission": dataSourceFronteggPermission(),
			},
			ResourcesMap: map[string]*schema.Resource{
				"frontegg_permission":          resourceFronteggPermission(),
				"frontegg_permission_category": resourceFronteggPermissionCategory(),
				"frontegg_role":                resourceFronteggRole(),
				"frontegg_webhook":             resourceFronteggWebhook(),
				"frontegg_workspace":           resourceFronteggWorkspace(),
			},
			ConfigureContextFunc: func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
				client := &restclient.Client{}
				{
					in := struct {
						ClientId  string `json:"clientId"`
						SecretKey string `json:"secret"`
					}{
						ClientId:  d.Get("client_id").(string),
						SecretKey: d.Get("secret_key").(string),
					}
					var out struct {
						AccessToken string `json:"accessToken"`
					}
					err := client.Post(ctx, "https://portal.frontegg.com/frontegg/identity/resources/auth/v1/api-token", in, &out)
					if err != nil {
						return nil, diag.Errorf("unable to authenticate with frontegg: %s", err)
					}
					client.Authenticate(out.AccessToken)
				}
				return client, nil
			},
		}
	}
}
