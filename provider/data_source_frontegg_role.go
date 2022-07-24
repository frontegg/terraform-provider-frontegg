package provider

import (
	"context"

	"github.com/benesch/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceFronteggRole() *schema.Resource {
	s := resourceFronteggRole().Schema
	for _, field := range s {
		field.Required = false
		field.Computed = true
	}
	s["key"].Computed = false
	s["key"].Required = true
	return &schema.Resource{
		ReadContext: dataSourceFronteggRoleRead,
		Schema:      s,
	}
}

func dataSourceFronteggRoleRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*restclient.Client)
	var out []fronteggRole
	if err := client.Get(ctx, fronteggRolePath, &out); err != nil {
		return diag.FromErr(err)
	}
	key := d.Get("key").(string)
	for _, c := range out {
		if c.Key == key {
			if err := resourceFronteggRoleDeserialize(d, c); err != nil {
				return diag.FromErr(err)
			}
			return diag.Diagnostics{}
		}
	}
	return diag.Errorf("unable to find Role with key %s", key)
}
