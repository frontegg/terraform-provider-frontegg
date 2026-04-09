package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/frontegg/terraform-provider-frontegg/provider/validators"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggTenantAPITokenPath = "/identity/resources/tenants/api-tokens/v1"

// fronteggTenantAPITokenCreateRequest is the POST request body.
// ExpiresInMinutes is only valid on create — the API returns the absolute
// expiry timestamp on read and does not accept it on PATCH.
type fronteggTenantAPITokenCreateRequest struct {
	Description      string                 `json:"description,omitempty"`
	RoleIDs          []string               `json:"roleIds,omitempty"`
	ExpiresInMinutes *int                   `json:"expiresInMinutes,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// fronteggTenantAPITokenUpdateRequest is the PATCH request body.
// Supports description, roleIds, and metadata. Expiry cannot be changed.
type fronteggTenantAPITokenUpdateRequest struct {
	Description string                 `json:"description"`
	RoleIDs     []string               `json:"roleIds"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// fronteggTenantAPIToken is the read/create/update response shape.
// Secret is only populated in the POST response — never in GET or PATCH.
type fronteggTenantAPIToken struct {
	ClientID    string   `json:"clientId"`
	Secret      string   `json:"secret,omitempty"`
	TenantID    string   `json:"tenantId"`
	Description string   `json:"description"`
	RoleIDs     []string `json:"roleIds"`
	Expires     string   `json:"expires"`
	CreatedAt   string   `json:"createdAt"`
}

func resourceFronteggTenantAPIToken() *schema.Resource {
	return &schema.Resource{
		Description: `Manages an API token for a Frontegg tenant. API tokens (client credentials) allow machine-to-machine authentication.

The token ` + "`secret`" + ` is returned only at creation time and is never retrievable again — store it immediately (e.g. in a secrets manager).

` + "`tenant_id`" + ` and ` + "`expires_in_minutes`" + ` are immutable and force replacement when changed. ` + "`description`" + `, ` + "`role_ids`" + `, and ` + "`metadata`" + ` can be updated in-place.

**Import note:** After import, the ` + "`secret`" + ` and ` + "`metadata`" + ` fields will be empty in state. Import format: ` + "`tenant_id:client_id`" + `.`,

		CreateContext: resourceFronteggTenantAPITokenCreate,
		ReadContext:   resourceFronteggTenantAPITokenRead,
		UpdateContext: resourceFronteggTenantAPITokenUpdate,
		DeleteContext: resourceFronteggTenantAPITokenDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourceFronteggTenantAPITokenImport,
		},

		Schema: map[string]*schema.Schema{
			"tenant_id": {
				Description: "The ID of the tenant that owns this API token. Changing this forces a new token to be created.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"description": {
				Description: "A human-readable description for the API token.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"role_ids": {
				Description: "List of role IDs to assign to this API token.",
				Type:        schema.TypeSet,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			"expires_in_minutes": {
				Description:  "Token expiration time in minutes (minimum 1). Omit for a non-expiring token. Changing this forces a new token to be created.",
				Type:         schema.TypeInt,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: validation.IntAtLeast(1),
			},
			"metadata": {
				Description:  "A JSON object of custom metadata to encode into the token's JWT claims. Write-only: not read back from the API after creation. After import, this field will be empty in state.",
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validators.ValidateJSON,
			},
			"client_id": {
				Description: "The client ID of the API token. Used for machine-to-machine authentication.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"secret": {
				Description: "The client secret of the API token. Only available at creation time — store it immediately. Cannot be retrieved after creation.",
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
			},
			"expires": {
				Description: "The expiration timestamp of the token (RFC3339). Empty if the token does not expire.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"created_at": {
				Description: "The timestamp when the token was created.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func resourceFronteggTenantAPITokenImport(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	parts := strings.SplitN(d.Id(), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid import format, expected tenant_id:client_id, got: %s", d.Id())
	}
	if err := d.Set("tenant_id", parts[0]); err != nil {
		return nil, err
	}
	d.SetId(parts[1])
	return []*schema.ResourceData{d}, nil
}

func resourceFronteggTenantAPITokenCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	tenantID := d.Get("tenant_id").(string)
	if tenantID == "" {
		return diag.Errorf("tenant_id is required but is empty; use 'tenant_id:client_id' format when importing")
	}

	headers := http.Header{}
	headers.Add("frontegg-tenant-id", tenantID)

	in := fronteggTenantAPITokenCreateRequest{
		Description: d.Get("description").(string),
		RoleIDs:     stringSetToList(d.Get("role_ids").(*schema.Set)),
	}

	if v, ok := d.GetOk("expires_in_minutes"); ok {
		mins := v.(int)
		in.ExpiresInMinutes = &mins
	}

	if v, ok := d.GetOk("metadata"); ok && v.(string) != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(v.(string)), &metadata); err != nil {
			return diag.Errorf("metadata is not valid JSON: %s", err)
		}
		in.Metadata = metadata
	}

	var out fronteggTenantAPIToken
	if err := clientHolder.ApiClient.PostWithHeaders(ctx, fronteggTenantAPITokenPath, headers, in, &out); err != nil {
		return diag.FromErr(err)
	}

	if out.ClientID == "" {
		return diag.Errorf("API returned an empty clientId; the token may have been created but cannot be tracked — check the Frontegg console")
	}

	// Set the ID first so Terraform tracks this resource even if a subsequent
	// d.Set call fails.
	d.SetId(out.ClientID)

	// Store the secret immediately — it is never returned by the read API.
	if err := d.Set("secret", out.Secret); err != nil {
		return diag.FromErr(err)
	}

	// Delegate remaining computed fields to Read so state is fully normalised
	// from the API response. Read skips secret and metadata (write-only).
	return resourceFronteggTenantAPITokenRead(ctx, d, meta)
}

func resourceFronteggTenantAPITokenRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	tenantID := d.Get("tenant_id").(string)
	if tenantID == "" {
		return diag.Errorf("tenant_id is required but is empty; use 'tenant_id:client_id' format when importing")
	}

	headers := http.Header{}
	headers.Add("frontegg-tenant-id", tenantID)

	var tokens []fronteggTenantAPIToken
	if err := clientHolder.ApiClient.GetWithHeaders(ctx, fronteggTenantAPITokenPath, headers, &tokens); err != nil {
		return diag.FromErr(err)
	}

	var found *fronteggTenantAPIToken
	for i := range tokens {
		if tokens[i].ClientID == d.Id() {
			found = &tokens[i]
			break
		}
	}
	if found == nil {
		d.SetId("")
		return nil
	}

	if err := d.Set("client_id", found.ClientID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("description", found.Description); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("role_ids", found.RoleIDs); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("expires", found.Expires); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("created_at", found.CreatedAt); err != nil {
		return diag.FromErr(err)
	}
	// Do not set "secret" — not returned by the read API; preserved from creation state.
	// Do not set "metadata" — write-only; not returned by the read API.
	return nil
}

func resourceFronteggTenantAPITokenUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	tenantID := d.Get("tenant_id").(string)
	if tenantID == "" {
		return diag.Errorf("tenant_id is required but is empty")
	}

	headers := http.Header{}
	headers.Add("frontegg-tenant-id", tenantID)

	in := fronteggTenantAPITokenUpdateRequest{
		Description: d.Get("description").(string),
		RoleIDs:     stringSetToList(d.Get("role_ids").(*schema.Set)),
	}

	if v, ok := d.GetOk("metadata"); ok && v.(string) != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(v.(string)), &metadata); err != nil {
			return diag.Errorf("metadata is not valid JSON: %s", err)
		}
		in.Metadata = metadata
	}

	if err := clientHolder.ApiClient.PatchWithHeaders(
		ctx,
		fmt.Sprintf("%s/%s", fronteggTenantAPITokenPath, d.Id()),
		headers,
		in,
		nil,
	); err != nil {
		return diag.FromErr(err)
	}

	return resourceFronteggTenantAPITokenRead(ctx, d, meta)
}

func resourceFronteggTenantAPITokenDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	tenantID := d.Get("tenant_id").(string)
	if tenantID == "" {
		return diag.Errorf("tenant_id is required but is empty")
	}

	headers := http.Header{}
	headers.Add("frontegg-tenant-id", tenantID)

	if err := clientHolder.ApiClient.DeleteWithHeaders(
		ctx,
		fmt.Sprintf("%s/%s", fronteggTenantAPITokenPath, d.Id()),
		headers,
		nil,
	); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
