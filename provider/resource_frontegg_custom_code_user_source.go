package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type fronteggCustomCodeUserSourceConfig struct {
	SyncOnLogin        bool        `json:"syncOnLogin"`
	IsMigrated         bool        `json:"isMigrated"`
	TenantConfig       interface{} `json:"tenantConfig"`
	CodePayload        string      `json:"codePayload"`
	GetUserCodePayload string      `json:"getUserCodePayload,omitempty"`
}

type fronteggCustomCodeUserSourceRequest struct {
	Name          string                             `json:"name"`
	Configuration fronteggCustomCodeUserSourceConfig `json:"configuration"`
	AppIDs        []string                           `json:"appIds,omitempty"`
	Index         int                                `json:"index"`
	Description   string                             `json:"description,omitempty"`
}

const fronteggCustomCodeUserSourcePath = "/identity/resources/user-sources/v1/external/custom-code"

func resourceFronteggCustomCodeUserSource() *schema.Resource {
	baseSchema := userSourceBaseSchema()

	// Add Custom-Code specific fields
	baseSchema["code_payload"] = &schema.Schema{
		Description: "The custom code that will be executed to authenticate the user.",
		Type:        schema.TypeString,
		Required:    true,
	}

	baseSchema["get_user_code_payload"] = &schema.Schema{
		Description: "The custom code that will be executed to get user details.",
		Type:        schema.TypeString,
		Optional:    true,
	}

	return &schema.Resource{
		Description: `Configures a Frontegg Custom-Code user source.`,

		CreateContext: resourceFronteggCustomCodeUserSourceCreate,
		ReadContext:   resourceFronteggCustomCodeUserSourceRead,
		UpdateContext: resourceFronteggCustomCodeUserSourceUpdate,
		DeleteContext: resourceFronteggCustomCodeUserSourceDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: baseSchema,
	}
}

func resourceFronteggCustomCodeUserSourceSerialize(d *schema.ResourceData) (fronteggCustomCodeUserSourceRequest, error) {
	appIDs := extractAppIDs(d)

	tenantConfig, err := buildUserSourceTenantConfig(d)
	if err != nil {
		return fronteggCustomCodeUserSourceRequest{}, err
	}

	config := fronteggCustomCodeUserSourceConfig{
		SyncOnLogin:  d.Get("sync_on_login").(bool),
		IsMigrated:   d.Get("is_migrated").(bool),
		TenantConfig: tenantConfig,
		CodePayload:  d.Get("code_payload").(string),
	}

	// Add the optional getUserCodePayload
	if getUserCode, ok := d.GetOk("get_user_code_payload"); ok {
		config.GetUserCodePayload = getUserCode.(string)
	}

	return fronteggCustomCodeUserSourceRequest{
		Name:          d.Get("name").(string),
		Configuration: config,
		AppIDs:        appIDs,
		Index:         d.Get("index").(int),
		Description:   d.Get("description").(string),
	}, nil
}

func resourceFronteggCustomCodeUserSourceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in, err := resourceFronteggCustomCodeUserSourceSerialize(d)
	if err != nil {
		return diag.FromErr(err)
	}

	var out fronteggBaseUserSourceResponse
	if err := clientHolder.ApiClient.Post(ctx, fronteggCustomCodeUserSourcePath, in, &out); err != nil {
		return diag.FromErr(err)
	}

	if err := deserializeUserSourceResponse(d, out); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggCustomCodeUserSourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return readUserSource(ctx, d, meta, deserializeUserSourceResponse)
}

func resourceFronteggCustomCodeUserSourceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in, err := resourceFronteggCustomCodeUserSourceSerialize(d)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s", fronteggCustomCodeUserSourcePath, d.Id()), in, nil); err != nil {
		return diag.FromErr(err)
	}

	return resourceFronteggCustomCodeUserSourceRead(ctx, d, meta)
}

func resourceFronteggCustomCodeUserSourceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return deleteUserSource(ctx, d, meta)
}
