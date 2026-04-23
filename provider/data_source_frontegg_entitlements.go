package provider

import (
	"context"
	"net/url"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourceFronteggEntitlements() *schema.Resource {
	return &schema.Resource{
		Description: "Paginated lookup of Frontegg entitlements filtered by plan, tenant, user, or assignment level.",
		ReadContext: dataSourceFronteggEntitlementsRead,
		Schema: map[string]*schema.Schema{
			"plan_ids": {
				Description: "Filter by plan IDs.",
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			"tenant_ids": {
				Description: "Filter by tenant IDs.",
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			"user_ids": {
				Description: "Filter by user IDs.",
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			"assign_level": {
				Description:  "Restrict to TENANT- or USER-level entitlements.",
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{"TENANT", "USER"}, false),
			},
			"with_relations": {
				Description: "Include related plan data in each returned entitlement.",
				Type:        schema.TypeBool,
				Optional:    true,
			},
			"entitlements": {
				Description: "Matching entitlements.",
				Type:        schema.TypeList,
				Computed:    true,
				Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"id":              {Type: schema.TypeString, Computed: true},
					"plan_id":         {Type: schema.TypeString, Computed: true},
					"tenant_id":       {Type: schema.TypeString, Computed: true},
					"user_id":         {Type: schema.TypeString, Computed: true},
					"expiration_date": {Type: schema.TypeString, Computed: true},
					"created_at":      {Type: schema.TypeString, Computed: true},
					"updated_at":      {Type: schema.TypeString, Computed: true},
				}},
			},
		},
	}
}

func dataSourceFronteggEntitlementsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	c := &clientHolder.ApiClient

	filters := url.Values{}
	for _, spec := range []struct {
		field string
		param string
	}{
		{"plan_ids", "planIds"},
		{"tenant_ids", "tenantIds"},
		{"user_ids", "userIds"},
	} {
		if raw, ok := d.GetOk(spec.field); ok {
			for _, v := range raw.([]interface{}) {
				filters.Add(spec.param, v.(string))
			}
		}
	}
	if v, ok := d.GetOk("assign_level"); ok {
		filters.Set("assignLevel", v.(string))
	}
	if v, ok := d.GetOk("with_relations"); ok {
		if v.(bool) {
			filters.Set("withRelations", "true")
		}
	}

	entitlements, err := listEntitlements(ctx, c, filters)
	if err != nil {
		return diag.FromErr(err)
	}

	out := make([]map[string]interface{}, 0, len(entitlements))
	for _, e := range entitlements {
		out = append(out, map[string]interface{}{
			"id":              e.ID,
			"plan_id":         e.PlanID,
			"tenant_id":       e.TenantID,
			"user_id":         e.UserID,
			"expiration_date": e.ExpirationDate,
			"created_at":      e.CreatedAt,
			"updated_at":      e.UpdatedAt,
		})
	}

	if err := d.Set("entitlements", out); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(id.UniqueId())
	return nil
}
