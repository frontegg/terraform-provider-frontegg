package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Common paths for user sources
const fronteggUserSourceBasePath = "/identity/resources/user-sources/v1"

// Common response type for all user sources
type fronteggBaseUserSourceResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	AppIDs      []string `json:"appIds"`
	Description string   `json:"description"`
	Index       int      `json:"index"`
}

// Builds the tenant configuration based on the terraform resource data
func buildUserSourceTenantConfig(d *schema.ResourceData) (interface{}, error) {
	resolverType := d.Get("tenant_resolver_type").(string)

	switch resolverType {
	case "dynamic":
		fieldName := d.Get("tenant_id_field_name").(string)
		if fieldName == "" {
			return nil, fmt.Errorf("tenant_id_field_name is required when tenant_resolver_type is dynamic")
		}
		return map[string]interface{}{
			"tenantResolverType": "dynamic",
			"tenantIdFieldName":  fieldName,
		}, nil
	case "static":
		tenantID := d.Get("tenant_id").(string)
		if tenantID == "" {
			return nil, fmt.Errorf("tenant_id is required when tenant_resolver_type is static")
		}
		return map[string]interface{}{
			"tenantResolverType": "static",
			"tenantId":           tenantID,
		}, nil
	case "new":
		return map[string]interface{}{
			"tenantResolverType": "new",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported tenant_resolver_type: %s", resolverType)
	}
}

// Common schema fields for all user sources
func userSourceBaseSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {
			Description: "The user source name.",
			Type:        schema.TypeString,
			Required:    true,
		},
		"description": {
			Description: "The user source description.",
			Type:        schema.TypeString,
			Optional:    true,
		},
		"index": {
			Description: "The user source index.",
			Type:        schema.TypeInt,
			Required:    true,
		},
		"app_ids": {
			Description: "The application IDs to assign to this user source.",
			Type:        schema.TypeSet,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Optional: true,
		},
		"sync_on_login": {
			Description: "Whether to sync user profile attributes on each login.",
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     false,
		},
		"is_migrated": {
			Description: "Whether to migrate the users.",
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     false,
		},
		"tenant_resolver_type": {
			Description: "The tenant resolver type (dynamic, static, or new).",
			Type:        schema.TypeString,
			Required:    true,
			ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
				v := val.(string)
				validTypes := map[string]bool{
					"dynamic": true,
					"static":  true,
					"new":     true,
				}
				if !validTypes[v] {
					errs = append(errs, fmt.Errorf("%q must be one of 'dynamic', 'static', or 'new', got: %s", key, v))
				}
				return
			},
		},
		"tenant_id": {
			Description: "The tenant ID for static tenant resolver type.",
			Type:        schema.TypeString,
			Optional:    true,
		},
		"tenant_id_field_name": {
			Description: "The attribute name from which the tenant ID would be taken for dynamic tenant resolver type.",
			Type:        schema.TypeString,
			Optional:    true,
		},
	}
}

// Extract app IDs from schema resource data
func extractAppIDs(d *schema.ResourceData) []string {
	appIDsSet := d.Get("app_ids").(*schema.Set)
	var appIDs []string

	for _, appID := range appIDsSet.List() {
		appIDs = append(appIDs, appID.(string))
	}

	return appIDs
}

// Deserialize common user source response fields
func deserializeUserSourceResponse(d *schema.ResourceData, source fronteggBaseUserSourceResponse) error {
	d.SetId(source.ID)
	if err := d.Set("name", source.Name); err != nil {
		return err
	}
	if err := d.Set("description", source.Description); err != nil {
		return err
	}
	if err := d.Set("index", source.Index); err != nil {
		return err
	}
	if err := d.Set("app_ids", source.AppIDs); err != nil {
		return err
	}
	return nil
}

// Common Read function for all user sources
func readUserSource(ctx context.Context, d *schema.ResourceData, meta interface{}, deserializeFunc func(*schema.ResourceData, fronteggBaseUserSourceResponse) error) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	client := clientHolder.ApiClient
	client.Ignore404()

	var out fronteggBaseUserSourceResponse
	if err := client.Get(ctx, fmt.Sprintf("%s/%s", fronteggUserSourceBasePath, d.Id()), &out); err != nil {
		return diag.FromErr(err)
	}

	if out.ID == "" {
		d.SetId("")
		return nil
	}

	if err := deserializeFunc(d, out); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

// Common Delete function for all user sources
func deleteUserSource(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggUserSourceBasePath, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
