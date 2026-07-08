package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type fronteggFederationOauth2Config struct {
	AuthorizationURL     string   `json:"authorizationUrl"`
	TokenURL             string   `json:"tokenUrl"`
	UserInfoURL          string   `json:"userInfoUrl"`
	Scopes               []string `json:"scopes,omitempty"`
	GrantTypes           []string `json:"grantTypes,omitempty"`
	CodeChallengeMethods []string `json:"codeChallengeMethods,omitempty"`
}

type fronteggFederationUserSourceConfig struct {
	SyncOnLogin  bool                            `json:"syncOnLogin"`
	TenantConfig interface{}                     `json:"tenantConfig"`
	WellknownURL string                          `json:"wellknownUrl,omitempty"`
	Oauth2Config *fronteggFederationOauth2Config `json:"oauth2Config,omitempty"`
	ClientID     string                          `json:"clientId"`
	Secret       string                          `json:"secret,omitempty"`
	UsePkce      bool                            `json:"usePkce,omitempty"`
}

type fronteggFederationUserSourceRequest struct {
	Name          string                             `json:"name"`
	Configuration fronteggFederationUserSourceConfig `json:"configuration"`
	AppIDs        []string                           `json:"appIds,omitempty"`
	Index         int                                `json:"index,omitempty"`
	Description   string                             `json:"description,omitempty"`
}

const fronteggFederationUserSourcePath = "/identity/resources/user-sources/v1/federation"

func federationStringListSchema(description string) *schema.Schema {
	return &schema.Schema{
		Description: description,
		Type:        schema.TypeList,
		Optional:    true,
		Elem:        &schema.Schema{Type: schema.TypeString},
	}
}

func resourceFronteggFederationUserSource() *schema.Resource {
	baseSchema := userSourceBaseSchema()

	// Federation config does not have an isMigrated flag.
	delete(baseSchema, "is_migrated")

	// index is not required for federation sources.
	baseSchema["index"] = &schema.Schema{
		Description: "The user source index.",
		Type:        schema.TypeInt,
		Optional:    true,
	}

	baseSchema["wellknown_url"] = &schema.Schema{
		Description: "The well-known URL of the OIDC provider. Required if oauth2_config is not provided.",
		Type:        schema.TypeString,
		Optional:    true,
	}
	baseSchema["oauth2_config"] = &schema.Schema{
		Description: "OAuth2 configuration. Required if wellknown_url is not provided.",
		Type:        schema.TypeList,
		Optional:    true,
		MaxItems:    1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"authorization_url": {
					Description: "The authorization URL of the OAuth2 provider.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"token_url": {
					Description: "The token URL of the OAuth2 provider.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"user_info_url": {
					Description: "The user info URL of the OAuth2 provider.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"scopes":                 federationStringListSchema("The scopes to request from the OAuth2 provider."),
				"grant_types":            federationStringListSchema("The OAuth2 grant types."),
				"code_challenge_methods": federationStringListSchema("The PKCE code challenge methods."),
			},
		},
	}
	baseSchema["client_id"] = &schema.Schema{
		Description: "The client id from the identity provider.",
		Type:        schema.TypeString,
		Required:    true,
	}
	baseSchema["secret"] = &schema.Schema{
		Description: "The secret from the identity provider. Required if use_pkce is not enabled.",
		Type:        schema.TypeString,
		Optional:    true,
		Sensitive:   true,
	}
	baseSchema["use_pkce"] = &schema.Schema{
		Description: "Whether to use PKCE.",
		Type:        schema.TypeBool,
		Optional:    true,
		Default:     false,
	}

	return &schema.Resource{
		Description: `Configures a Frontegg federation (OIDC/OAuth2) user source.`,

		CreateContext: resourceFronteggFederationUserSourceCreate,
		ReadContext:   resourceFronteggFederationUserSourceRead,
		UpdateContext: resourceFronteggFederationUserSourceUpdate,
		DeleteContext: resourceFronteggFederationUserSourceDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: baseSchema,
	}
}

func extractFederationStringList(m map[string]interface{}, key string) []string {
	raw, ok := m[key].([]interface{})
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		out = append(out, v.(string))
	}
	return out
}

func extractFederationOauth2Config(d *schema.ResourceData) *fronteggFederationOauth2Config {
	list := d.Get("oauth2_config").([]interface{})
	if len(list) == 0 || list[0] == nil {
		return nil
	}
	m := list[0].(map[string]interface{})
	return &fronteggFederationOauth2Config{
		AuthorizationURL:     m["authorization_url"].(string),
		TokenURL:             m["token_url"].(string),
		UserInfoURL:          m["user_info_url"].(string),
		Scopes:               extractFederationStringList(m, "scopes"),
		GrantTypes:           extractFederationStringList(m, "grant_types"),
		CodeChallengeMethods: extractFederationStringList(m, "code_challenge_methods"),
	}
}

func resourceFronteggFederationUserSourceSerialize(d *schema.ResourceData) (fronteggFederationUserSourceRequest, error) {
	appIDs := extractAppIDs(d)

	tenantConfig, err := buildUserSourceTenantConfig(d)
	if err != nil {
		return fronteggFederationUserSourceRequest{}, err
	}

	wellknownURL := d.Get("wellknown_url").(string)
	oauth2Config := extractFederationOauth2Config(d)
	if wellknownURL == "" && oauth2Config == nil {
		return fronteggFederationUserSourceRequest{}, fmt.Errorf("one of wellknown_url or oauth2_config must be set")
	}

	usePkce := d.Get("use_pkce").(bool)
	secret := d.Get("secret").(string)
	if !usePkce && secret == "" {
		return fronteggFederationUserSourceRequest{}, fmt.Errorf("secret is required when use_pkce is not enabled")
	}

	config := fronteggFederationUserSourceConfig{
		SyncOnLogin:  d.Get("sync_on_login").(bool),
		TenantConfig: tenantConfig,
		WellknownURL: wellknownURL,
		Oauth2Config: oauth2Config,
		ClientID:     d.Get("client_id").(string),
		Secret:       secret,
		UsePkce:      usePkce,
	}

	return fronteggFederationUserSourceRequest{
		Name:          d.Get("name").(string),
		Configuration: config,
		AppIDs:        appIDs,
		Index:         d.Get("index").(int),
		Description:   d.Get("description").(string),
	}, nil
}

func resourceFronteggFederationUserSourceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in, err := resourceFronteggFederationUserSourceSerialize(d)
	if err != nil {
		return diag.FromErr(err)
	}

	var out fronteggBaseUserSourceResponse
	if err := clientHolder.ApiClient.Post(ctx, fronteggFederationUserSourcePath, in, &out); err != nil {
		return diag.FromErr(err)
	}

	if err := deserializeUserSourceResponse(d, out); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggFederationUserSourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return readUserSource(ctx, d, meta, deserializeUserSourceResponse)
}

func resourceFronteggFederationUserSourceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in, err := resourceFronteggFederationUserSourceSerialize(d)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s", fronteggFederationUserSourcePath, d.Id()), in, nil); err != nil {
		return diag.FromErr(err)
	}

	return resourceFronteggFederationUserSourceRead(ctx, d, meta)
}

func resourceFronteggFederationUserSourceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return deleteUserSource(ctx, d, meta)
}
