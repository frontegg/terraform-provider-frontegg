package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type fronteggCognitoUserSourceConfig struct {
	SyncOnLogin     bool        `json:"syncOnLogin"`
	IsMigrated      bool        `json:"isMigrated"`
	Region          string      `json:"region"`
	ClientID        string      `json:"clientId"`
	UserPoolID      string      `json:"userPoolId"`
	AccessKeyID     string      `json:"accessKeyId"`
	SecretAccessKey string      `json:"secretAccessKey"`
	ClientSecret    string      `json:"clientSecret,omitempty"`
	TenantConfig    interface{} `json:"tenantConfig"`
}

type fronteggCognitoUserSourceRequest struct {
	Name          string                          `json:"name"`
	Configuration fronteggCognitoUserSourceConfig `json:"configuration"`
	AppIDs        []string                        `json:"appIds,omitempty"`
	Index         int                             `json:"index"`
	Description   string                          `json:"description,omitempty"`
}

const fronteggCognitoUserSourcePath = "/identity/resources/user-sources/v1/external/cognito"

func resourceFronteggCognitoUserSource() *schema.Resource {
	baseSchema := userSourceBaseSchema()

	// Add Cognito-specific fields
	baseSchema["region"] = &schema.Schema{
		Description: "The AWS region of the Cognito user pool.",
		Type:        schema.TypeString,
		Required:    true,
	}
	baseSchema["client_id"] = &schema.Schema{
		Description: "The Cognito app client ID.",
		Type:        schema.TypeString,
		Required:    true,
	}
	baseSchema["user_pool_id"] = &schema.Schema{
		Description: "The ID of the Cognito user pool.",
		Type:        schema.TypeString,
		Required:    true,
	}
	baseSchema["access_key_id"] = &schema.Schema{
		Description: "The access key of the AWS account.",
		Type:        schema.TypeString,
		Required:    true,
	}
	baseSchema["secret_access_key"] = &schema.Schema{
		Description: "The secret of the AWS account.",
		Type:        schema.TypeString,
		Required:    true,
		Sensitive:   true,
	}
	baseSchema["client_secret"] = &schema.Schema{
		Description: "The Cognito application client secret, required if the app client is configured with a client secret.",
		Type:        schema.TypeString,
		Optional:    true,
		Sensitive:   true,
	}

	return &schema.Resource{
		Description: `Configures a Frontegg Cognito user source.`,

		CreateContext: resourceFronteggCognitoUserSourceCreate,
		ReadContext:   resourceFronteggCognitoUserSourceRead,
		UpdateContext: resourceFronteggCognitoUserSourceUpdate,
		DeleteContext: resourceFronteggCognitoUserSourceDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: baseSchema,
	}
}

func resourceFronteggCognitoUserSourceSerialize(d *schema.ResourceData) (fronteggCognitoUserSourceRequest, error) {
	appIDs := extractAppIDs(d)

	tenantConfig, err := buildUserSourceTenantConfig(d)
	if err != nil {
		return fronteggCognitoUserSourceRequest{}, err
	}

	config := fronteggCognitoUserSourceConfig{
		SyncOnLogin:     d.Get("sync_on_login").(bool),
		IsMigrated:      d.Get("is_migrated").(bool),
		Region:          d.Get("region").(string),
		ClientID:        d.Get("client_id").(string),
		UserPoolID:      d.Get("user_pool_id").(string),
		AccessKeyID:     d.Get("access_key_id").(string),
		SecretAccessKey: d.Get("secret_access_key").(string),
		ClientSecret:    d.Get("client_secret").(string),
		TenantConfig:    tenantConfig,
	}

	return fronteggCognitoUserSourceRequest{
		Name:          d.Get("name").(string),
		Configuration: config,
		AppIDs:        appIDs,
		Index:         d.Get("index").(int),
		Description:   d.Get("description").(string),
	}, nil
}

func resourceFronteggCognitoUserSourceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in, err := resourceFronteggCognitoUserSourceSerialize(d)
	if err != nil {
		return diag.FromErr(err)
	}

	var out fronteggBaseUserSourceResponse
	if err := clientHolder.ApiClient.Post(ctx, fronteggCognitoUserSourcePath, in, &out); err != nil {
		return diag.FromErr(err)
	}

	if err := deserializeUserSourceResponse(d, out); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggCognitoUserSourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return readUserSource(ctx, d, meta, deserializeUserSourceResponse)
}

func resourceFronteggCognitoUserSourceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in, err := resourceFronteggCognitoUserSourceSerialize(d)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s", fronteggCognitoUserSourcePath, d.Id()), in, nil); err != nil {
		return diag.FromErr(err)
	}

	return resourceFronteggCognitoUserSourceRead(ctx, d, meta)
}

func resourceFronteggCognitoUserSourceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return deleteUserSource(ctx, d, meta)
}
