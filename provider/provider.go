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
				"api_base_url": {
					Description: "The Frontegg api url. Override to change region. Defaults to EU url.",
					Type:        schema.TypeString,
					Optional:    true,
					Default:     "https://api.frontegg.com",
					DefaultFunc: schema.EnvDefaultFunc("FRONTEGG_API_BASE_URL", nil),
				},
				"portal_base_url": {
					Description: "The Frontegg portal url. Override to change region. Defaults to EU url.",
					Type:        schema.TypeString,
					Optional:    true,
					Default:     "https://portal.frontegg.com",
					DefaultFunc: schema.EnvDefaultFunc("FRONTEGG_PORTAL_BASE_URL", nil),
				},
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
				"frontegg_tenant":              resourceFronteggTenant(),
				"frontegg_redirect_uri":        resourceFronteggRedirectUri(),
				"frontegg_allowed_origin":      resourceFronteggAllowedOrigin(),
			},
			ConfigureContextFunc: func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
				apiClient := restclient.MakeRestClient(d.Get("api_base_url").(string))
				portalClient := restclient.MakeRestClient(d.Get("portal_base_url").(string))
				{
					in := struct {
						ClientId  string `json:"clientId"`
						SecretKey string `json:"secret"`
					}{
						ClientId:  d.Get("client_id").(string),
						SecretKey: d.Get("secret_key").(string),
					}
					var out struct {
						AccessToken string `json:"token"`
					}
					err := apiClient.Post(ctx, "/auth/vendor", in, &out)
					if err != nil {
						return nil, diag.Errorf("unable to authenticate with frontegg: %s", err)
					}
					portalClient.Authenticate(out.AccessToken)
					apiClient.Authenticate(out.AccessToken)
				}
				return &restclient.ClientHolder{
					ApiClient:    apiClient,
					PortalClient: portalClient,
				}, nil
			},
		}
	}
}
