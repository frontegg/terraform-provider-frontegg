package provider

import (
	"context"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceFronteggPlan() *schema.Resource {
	s := resourceFronteggPlan().Schema
	// feature_keys is omitted from the data source: the plan GET response does
	// not return feature keys and resourceFronteggPlanDeserialize intentionally
	// preserves the existing state value for that field. A data source has no
	// prior state, so the attribute would always materialize as empty/null and
	// mislead consumers that reference data.frontegg_plan.*.feature_keys.
	delete(s, "feature_keys")
	// Force every field to pure read-only. The resource schema marks several fields
	// Optional (default_treatment, rules, description, default_time_limitation,
	// assign_on_signup) — leaving Optional set would advertise them as
	// user-settable in the data source.
	for _, field := range s {
		field.Required = false
		field.Optional = false
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
