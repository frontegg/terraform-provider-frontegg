package provider

import (
	"context"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceFronteggPlan() *schema.Resource {
	s := resourceFronteggPlan().Schema
	for _, field := range s {
		field.Required = false
		field.Computed = true
		field.Default = nil
		field.ValidateFunc = nil
	}
	s["name"].Computed = false
	s["name"].Required = true
	return &schema.Resource{
		Description: "Looks up a Frontegg plan by name and exposes its id and other attributes for composition with other resources (e.g. frontegg_entitlement).",
		ReadContext: dataSourceFronteggPlanRead,
		Schema:      s,
	}
}

func dataSourceFronteggPlanRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	name := d.Get("name").(string)
	plan, err := fetchFronteggPlanByName(ctx, name, clientHolder)
	if err != nil {
		return diag.FromErr(err)
	}
	if plan == nil {
		return diag.Errorf("unable to find plan with name %q", name)
	}
	if err := resourceFronteggPlanDeserialize(d, *plan); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
