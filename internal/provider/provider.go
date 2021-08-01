package provider

import (
	"context"

	"github.com/benesch/terraform-provider-frontegg/internal/restclient"
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
					Description: "The ID of the Frontegg workspace to operate on.",
					Type:        schema.TypeString,
					Required:    true,
					DefaultFunc: schema.EnvDefaultFunc("FRONTEGG_CLIENT_ID", nil),
				},
				"email": {
					Description: "The email of the Frontegg user to authenticate as.",
					Type:        schema.TypeString,
					Required:    true,
					DefaultFunc: schema.EnvDefaultFunc("FRONTEGG_EMAIL", nil),
				},
				"password": {
					Description: "The password of the Frontegg user to authenticate as.",
					Type:        schema.TypeString,
					Required:    true,
					Sensitive:   true,
					DefaultFunc: schema.EnvDefaultFunc("FRONTEGG_PASSWORD", nil),
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
						Email    string `json:"email"`
						Password string `json:"password"`
					}{
						Email:    d.Get("email").(string),
						Password: d.Get("password").(string),
					}
					var out struct {
						AccessToken string `json:"accessToken"`
					}
					err := client.Post(ctx, "https://portal.frontegg.com/frontegg/identity/resources/auth/v1/user", in, &out)
					if err != nil {
						return nil, diag.Errorf("unable to authenticate with frontegg: %s", err)
					}
					client.Authenticate(out.AccessToken)
				}
				{
					in := struct {
						TenantID string `json:"tenantId"`
					}{
						TenantID: d.Get("client_id").(string),
					}
					err := client.Put(ctx, "https://portal.frontegg.com/frontegg/identity/resources/users/v1/tenant", in, nil)
					if err != nil {
						return nil, diag.Errorf("unable to select frontegg tenant: %s", err)
					}
				}
				return client, nil
			},
		}
	}
}
