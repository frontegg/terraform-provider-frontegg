package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type fronteggAuth0UserSourceConfig struct {
	SyncOnLogin  bool        `json:"syncOnLogin"`
	IsMigrated   bool        `json:"isMigrated"`
	Domain       string      `json:"domain"`
	ClientID     string      `json:"clientId"`
	Secret       string      `json:"secret"`
	TenantConfig interface{} `json:"tenantConfig"`
}

type fronteggAuth0UserSourceRequest struct {
	Name          string                        `json:"name"`
	Configuration fronteggAuth0UserSourceConfig `json:"configuration"`
	AppIDs        []string                      `json:"appIds,omitempty"`
	Index         int                           `json:"index"`
	Description   string                        `json:"description,omitempty"`
}

const fronteggAuth0UserSourcePath = "/identity/resources/user-sources/v1/external/auth0"

func resourceFronteggAuth0UserSource() *schema.Resource {
	baseSchema := userSourceBaseSchema()

	// Add Auth0-specific fields
	baseSchema["domain"] = &schema.Schema{
		Description: "The Auth0 domain.",
		Type:        schema.TypeString,
		Required:    true,
	}
	baseSchema["client_id"] = &schema.Schema{
		Description: "The Auth0 application client ID.",
		Type:        schema.TypeString,
		Required:    true,
	}
	baseSchema["secret"] = &schema.Schema{
		Description: "The Auth0 application secret.",
		Type:        schema.TypeString,
		Required:    true,
		Sensitive:   true,
	}

	return &schema.Resource{
		Description: `Configures a Frontegg Auth0 user source.`,

		CreateContext: resourceFronteggAuth0UserSourceCreate,
		ReadContext:   resourceFronteggAuth0UserSourceRead,
		UpdateContext: resourceFronteggAuth0UserSourceUpdate,
		DeleteContext: resourceFronteggAuth0UserSourceDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: baseSchema,
	}
}

func resourceFronteggAuth0UserSourceSerialize(d *schema.ResourceData) (fronteggAuth0UserSourceRequest, error) {
	appIDs := extractAppIDs(d)

	tenantConfig, err := buildUserSourceTenantConfig(d)
	if err != nil {
		return fronteggAuth0UserSourceRequest{}, err
	}

	config := fronteggAuth0UserSourceConfig{
		SyncOnLogin:  d.Get("sync_on_login").(bool),
		IsMigrated:   d.Get("is_migrated").(bool),
		Domain:       d.Get("domain").(string),
		ClientID:     d.Get("client_id").(string),
		Secret:       d.Get("secret").(string),
		TenantConfig: tenantConfig,
	}

	return fronteggAuth0UserSourceRequest{
		Name:          d.Get("name").(string),
		Configuration: config,
		AppIDs:        appIDs,
		Index:         d.Get("index").(int),
		Description:   d.Get("description").(string),
	}, nil
}

func resourceFronteggAuth0UserSourceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in, err := resourceFronteggAuth0UserSourceSerialize(d)
	if err != nil {
		return diag.FromErr(err)
	}

	var out fronteggBaseUserSourceResponse
	if err := clientHolder.ApiClient.Post(ctx, fronteggAuth0UserSourcePath, in, &out); err != nil {
		return diag.FromErr(err)
	}

	if err := deserializeUserSourceResponse(d, out); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggAuth0UserSourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return readUserSource(ctx, d, meta, deserializeUserSourceResponse)
}

func resourceFronteggAuth0UserSourceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in, err := resourceFronteggAuth0UserSourceSerialize(d)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s", fronteggAuth0UserSourcePath, d.Id()), in, nil); err != nil {
		return diag.FromErr(err)
	}

	return resourceFronteggAuth0UserSourceRead(ctx, d, meta)
}

func resourceFronteggAuth0UserSourceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return deleteUserSource(ctx, d, meta)
}
