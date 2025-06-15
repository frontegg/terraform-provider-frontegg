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
					Default:     "https://frontegg-prod.frontegg.com",
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
				"environment_id": {
					Description: "The client ID from environment settings.",
					Type:        schema.TypeString,
					Optional:    true,
					Sensitive:   true,
				},
			},
			DataSourcesMap: map[string]*schema.Resource{
				"frontegg_permission": dataSourceFronteggPermission(),
			},
			ResourcesMap: map[string]*schema.Resource{
				"frontegg_permission":                    resourceFronteggPermission(),
				"frontegg_permission_category":           resourceFronteggPermissionCategory(),
				"frontegg_role":                          resourceFronteggRole(),
				"frontegg_webhook":                       resourceFronteggWebhook(),
				"frontegg_workspace":                     resourceFronteggWorkspace(),
				"frontegg_auth_policy":                   resourceFronteggAuthPolicy(),
				"frontegg_admin_portal":                  resourceFronteggAdminPortal(),
				"frontegg_tenant":                        resourceFronteggTenant(),
				"frontegg_user":                          resourceFronteggUser(),
				"frontegg_redirect_uri":                  resourceFronteggRedirectUri(),
				"frontegg_allowed_origin":                resourceFronteggAllowedOrigin(),
				"frontegg_email_provider":                resourceFronteggEmailProvider(),
				"frontegg_application":                   resourceFronteggApplication(),
				"frontegg_application_tenant_assignment": resourceFronteggApplicationTenantAssignment(),
				"frontegg_auth0_user_source":             resourceFronteggAuth0UserSource(),
				"frontegg_cognito_user_source":           resourceFronteggCognitoUserSource(),
				"frontegg_firebase_user_source":          resourceFronteggFirebaseUserSource(),
				"frontegg_custom_code_user_source":       resourceFronteggCustomCodeUserSource(),
				"frontegg_feature":                       resourceFronteggFeature(),
				"frontegg_plan":                          resourceFronteggPlan(),
				"frontegg_plan_feature":                  resourceFronteggPlanFeature(),
				"frontegg_secret":                        resourceFronteggSecret(),
			},
			ConfigureContextFunc: func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
				environmentId := d.Get("environment_id").(string)
				apiClient := restclient.MakeRestClient(d.Get("api_base_url").(string), environmentId)
				portalClient := restclient.MakeRestClient(d.Get("portal_base_url").(string), environmentId)
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
