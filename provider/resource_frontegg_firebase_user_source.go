package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type fronteggFirebaseServiceAccountConfig struct {
	Type           string `json:"type"`
	ProjectID      string `json:"project_id"`
	PrivateKeyID   string `json:"private_key_id"`
	PrivateKey     string `json:"private_key"`
	ClientEmail    string `json:"client_email"`
	ClientID       string `json:"client_id"`
	AuthURI        string `json:"auth_uri,omitempty"`
	TokenURI       string `json:"token_uri,omitempty"`
	AuthProvider   string `json:"auth_provider_x509_cert_url,omitempty"`
	ClientCert     string `json:"client_x509_cert_url,omitempty"`
	UniverseDomain string `json:"universe_domain"`
}

type fronteggFirebaseUserSourceConfig struct {
	SyncOnLogin    bool                                 `json:"syncOnLogin"`
	IsMigrated     bool                                 `json:"isMigrated"`
	TenantConfig   interface{}                          `json:"tenantConfig"`
	APIKey         string                               `json:"apiKey"`
	ServiceAccount fronteggFirebaseServiceAccountConfig `json:"serviceAccount"`
}

type fronteggFirebaseUserSourceRequest struct {
	Name          string                           `json:"name"`
	Configuration fronteggFirebaseUserSourceConfig `json:"configuration"`
	AppIDs        []string                         `json:"appIds,omitempty"`
	Index         int                              `json:"index"`
	Description   string                           `json:"description,omitempty"`
}

const fronteggFirebaseUserSourcePath = "/identity/resources/user-sources/v1/external/firebase"

func resourceFronteggFirebaseUserSource() *schema.Resource {
	baseSchema := userSourceBaseSchema()

	// Add Firebase-specific fields
	baseSchema["api_key"] = &schema.Schema{
		Description: "The Firebase Web API Key.",
		Type:        schema.TypeString,
		Required:    true,
	}
	baseSchema["service_account_type"] = &schema.Schema{
		Description: "Firebase service account type.",
		Type:        schema.TypeString,
		Required:    true,
	}
	baseSchema["project_id"] = &schema.Schema{
		Description: "Firebase project ID.",
		Type:        schema.TypeString,
		Required:    true,
	}
	baseSchema["private_key_id"] = &schema.Schema{
		Description: "Firebase service account private key ID.",
		Type:        schema.TypeString,
		Required:    true,
	}
	baseSchema["private_key"] = &schema.Schema{
		Description: "Firebase service account private key.",
		Type:        schema.TypeString,
		Required:    true,
		Sensitive:   true,
	}
	baseSchema["client_email"] = &schema.Schema{
		Description: "Firebase service account client email.",
		Type:        schema.TypeString,
		Required:    true,
	}
	baseSchema["client_id"] = &schema.Schema{
		Description: "Firebase service account client ID.",
		Type:        schema.TypeString,
		Required:    true,
	}
	baseSchema["auth_uri"] = &schema.Schema{
		Description: "Firebase service account auth URI.",
		Type:        schema.TypeString,
		Optional:    true,
	}
	baseSchema["token_uri"] = &schema.Schema{
		Description: "Firebase service account token URI.",
		Type:        schema.TypeString,
		Optional:    true,
	}
	baseSchema["auth_provider_x509_cert_url"] = &schema.Schema{
		Description: "Firebase service account auth provider x509 cert URL.",
		Type:        schema.TypeString,
		Optional:    true,
	}
	baseSchema["client_x509_cert_url"] = &schema.Schema{
		Description: "Firebase service account client x509 cert URL.",
		Type:        schema.TypeString,
		Optional:    true,
	}
	baseSchema["universe_domain"] = &schema.Schema{
		Description: "Firebase service account universe domain.",
		Type:        schema.TypeString,
		Required:    true,
	}

	return &schema.Resource{
		Description: `Configures a Frontegg Firebase user source.`,

		CreateContext: resourceFronteggFirebaseUserSourceCreate,
		ReadContext:   resourceFronteggFirebaseUserSourceRead,
		UpdateContext: resourceFronteggFirebaseUserSourceUpdate,
		DeleteContext: resourceFronteggFirebaseUserSourceDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: baseSchema,
	}
}

func resourceFronteggFirebaseUserSourceSerialize(d *schema.ResourceData) (fronteggFirebaseUserSourceRequest, error) {
	appIDs := extractAppIDs(d)

	tenantConfig, err := buildUserSourceTenantConfig(d)
	if err != nil {
		return fronteggFirebaseUserSourceRequest{}, err
	}

	serviceAccountConfig := fronteggFirebaseServiceAccountConfig{
		Type:           d.Get("service_account_type").(string),
		ProjectID:      d.Get("project_id").(string),
		PrivateKeyID:   d.Get("private_key_id").(string),
		PrivateKey:     d.Get("private_key").(string),
		ClientEmail:    d.Get("client_email").(string),
		ClientID:       d.Get("client_id").(string),
		AuthURI:        d.Get("auth_uri").(string),
		TokenURI:       d.Get("token_uri").(string),
		AuthProvider:   d.Get("auth_provider_x509_cert_url").(string),
		ClientCert:     d.Get("client_x509_cert_url").(string),
		UniverseDomain: d.Get("universe_domain").(string),
	}

	config := fronteggFirebaseUserSourceConfig{
		SyncOnLogin:    d.Get("sync_on_login").(bool),
		IsMigrated:     d.Get("is_migrated").(bool),
		APIKey:         d.Get("api_key").(string),
		TenantConfig:   tenantConfig,
		ServiceAccount: serviceAccountConfig,
	}

	return fronteggFirebaseUserSourceRequest{
		Name:          d.Get("name").(string),
		Configuration: config,
		AppIDs:        appIDs,
		Index:         d.Get("index").(int),
		Description:   d.Get("description").(string),
	}, nil
}

func resourceFronteggFirebaseUserSourceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in, err := resourceFronteggFirebaseUserSourceSerialize(d)
	if err != nil {
		return diag.FromErr(err)
	}

	var out fronteggBaseUserSourceResponse
	if err := clientHolder.ApiClient.Post(ctx, fronteggFirebaseUserSourcePath, in, &out); err != nil {
		return diag.FromErr(err)
	}

	if err := deserializeUserSourceResponse(d, out); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggFirebaseUserSourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return readUserSource(ctx, d, meta, deserializeUserSourceResponse)
}

func resourceFronteggFirebaseUserSourceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in, err := resourceFronteggFirebaseUserSourceSerialize(d)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s", fronteggFirebaseUserSourcePath, d.Id()), in, nil); err != nil {
		return diag.FromErr(err)
	}

	return resourceFronteggFirebaseUserSourceRead(ctx, d, meta)
}

func resourceFronteggFirebaseUserSourceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return deleteUserSource(ctx, d, meta)
}
