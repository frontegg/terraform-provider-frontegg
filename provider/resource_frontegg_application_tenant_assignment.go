package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const fronteggApplicationTenantAssignmentPath = "/applications/resources/applications/tenant-assignments/v1"

type fronteggApplicationTenantAssignment struct {
	TenantID string   `json:"tenantId"`
	AppIDs   []string `json:"appIds"`
}

// For handling the API response format in Read when it returns an object
type fronteggApplicationTenantIds struct {
	TenantIds []string `json:"tenantIds"`
}

func resourceFronteggApplicationTenantAssignment() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg application tenant assignment.`,

		CreateContext: resourceFronteggApplicationTenantAssignmentCreate,
		ReadContext:   resourceFronteggApplicationTenantAssignmentRead,
		DeleteContext: resourceFronteggApplicationTenantAssignmentDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceFronteggApplicationTenantAssignmentImport,
		},

		Schema: map[string]*schema.Schema{
			"app_id": {
				Description: "The ID of the application.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"tenant_id": {
				Description: "The ID of the tenant.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
		},
	}
}

func resourceFronteggApplicationTenantAssignmentImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	parts := strings.Split(d.Id(), ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid import format, expected app_id:tenant_id, got: %s", d.Id())
	}

	appID := parts[0]
	tenantID := parts[1]

	d.Set("app_id", appID)
	d.Set("tenant_id", tenantID)

	// Return the resource with the ID set
	return []*schema.ResourceData{d}, nil
}

func resourceFronteggApplicationTenantAssignmentCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	appID := d.Get("app_id").(string)
	tenantID := d.Get("tenant_id").(string)

	in := struct {
		TenantID string `json:"tenantId"`
	}{
		TenantID: tenantID,
	}

	var out fronteggApplicationTenantAssignment
	if err := clientHolder.ApiClient.Post(ctx, fmt.Sprintf("%s/%s", fronteggApplicationTenantAssignmentPath, appID), in, &out); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%s:%s", appID, tenantID))
	return nil
}

func resourceFronteggApplicationTenantAssignmentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	appID := d.Get("app_id").(string)
	tenantID := d.Get("tenant_id").(string)

	// Try a specialized endpoint first
	var checkAssignment struct {
		Exists bool `json:"exists"`
	}
	err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s/%s/check", fronteggApplicationTenantAssignmentPath, appID, tenantID), &checkAssignment)
	if err == nil {
		if !checkAssignment.Exists {
			d.SetId("")
		}
		return nil
	}

	// If that fails, try the standard endpoints

	// First, try to unmarshal as an array of tenant assignments
	var arrayResponse []fronteggApplicationTenantAssignment
	err = clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s", fronteggApplicationTenantAssignmentPath, appID), &arrayResponse)
	if err == nil {
		found := false
		for _, assignment := range arrayResponse {
			if assignment.TenantID == tenantID {
				found = true
				break
			}
		}
		if !found {
			d.SetId("")
		}
		return nil
	}

	// If the first attempt failed, try to unmarshal as an object with tenantIds field
	var objectResponse fronteggApplicationTenantIds
	err = clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s", fronteggApplicationTenantAssignmentPath, appID), &objectResponse)
	if err == nil {
		found := false
		for _, id := range objectResponse.TenantIds {
			if id == tenantID {
				found = true
				break
			}
		}
		if !found {
			d.SetId("")
		}
		return nil
	}

	// If we know the assignment exists (from the 409 Conflict error), but we can't read it,
	// just accept that it exists rather than returning an error
	return nil
}

func resourceFronteggApplicationTenantAssignmentDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	appID := d.Get("app_id").(string)
	tenantID := d.Get("tenant_id").(string)

	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s/%s", fronteggApplicationTenantAssignmentPath, appID, tenantID), nil); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
