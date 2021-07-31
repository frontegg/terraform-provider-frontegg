package provider

import (
	"context"

	"github.com/benesch/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceFronteggPermission() *schema.Resource {
	s := resourceFronteggPermission().Schema
	for _, field := range s {
		field.Required = false
		field.Computed = true
	}
	s["key"].Computed = false
	s["key"].Required = true
	return &schema.Resource{
		ReadContext: dataSourceFronteggPermissionRead,
		Schema:      s,
	}
}

func dataSourceFronteggPermissionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*restclient.Client)
	var out []fronteggPermission
	if err := client.Get(ctx, fronteggPermissionPath, &out); err != nil {
		return diag.FromErr(err)
	}
	key := d.Get("key").(string)
	for _, c := range out {
		if c.Key == key {
			if err := resourceFronteggPermissionDeserialize(d, c); err != nil {
				return diag.FromErr(err)
			}
			return diag.Diagnostics{}
		}
	}
	return diag.Errorf("unable to find permission with key %s", key)
}
